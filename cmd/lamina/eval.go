package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	test "github.com/benaskins/axon-test"
	"github.com/spf13/cobra"
)

var evalCmd = &cobra.Command{
	Use:   "eval <plan.yaml>",
	Short: "Run an evaluation plan against the chat service",
	Long:  `Load a YAML test plan, run each scenario against the chat service, grade responses, and print a report.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runEval,
}

func init() {
	rootCmd.AddCommand(evalCmd)
}

func runEval(cmd *cobra.Command, args []string) error {
	planPath := args[0]

	plan, err := test.LoadPlan(planPath)
	if err != nil {
		return fmt.Errorf("load plan: %w", err)
	}

	cfg := test.Config{
		AuthURL:      envOrDefault("AUTH_URL", "https://auth.studio.internal"),
		ChatURL:      envOrDefault("CHAT_URL", "https://chat.studio.internal"),
		AnalyticsURL: envOrDefault("ANALYTICS_URL", "https://analytics.studio.internal"),
	}

	client, err := test.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	// Set up optional LLM judge
	var judge test.Judge
	judgeModel := os.Getenv("JUDGE_MODEL")
	if judgeModel != "" {
		ollamaURL := envOrDefault("OLLAMA_URL", "http://localhost:11434")
		generate := newOllamaTextGenerator(ollamaURL, judgeModel)
		judge = test.NewOllamaJudge(generate)
	}

	fmt.Printf("━━━ %s ━━━\n", plan.Name)
	if judge != nil {
		fmt.Printf("Judge model: %s\n", judgeModel)
	}
	fmt.Println()

	// Build scenarios from plan
	scenarios := make([]test.Scenario, len(plan.Scenarios))
	for i, ps := range plan.Scenarios {
		scenarios[i] = test.Conversation(ps.Name, []test.Message{
			{Role: "user", Content: ps.Message},
		})
	}

	// Run all scenarios
	run, err := client.Run(plan.Name, scenarios)
	if err != nil {
		return fmt.Errorf("run scenarios: %w", err)
	}

	// Grade each scenario
	totalPassed, totalFailed := 0, 0
	for i, ps := range plan.Scenarios {
		if i >= len(run.Responses) {
			break
		}

		responses := run.Responses[i].Responses
		if len(responses) == 0 {
			continue
		}
		chatResult := responses[len(responses)-1]

		grade := test.GradeScenario(ps, chatResult, judge)
		printGrade(grade)

		totalPassed += grade.Passed
		totalFailed += grade.Failed
	}

	// Summary
	fmt.Println("━━━ Summary ━━━")
	fmt.Printf("  Run ID: %s\n", run.ID)
	fmt.Printf("  Criteria: %d passed, %d failed\n", totalPassed, totalFailed)

	if totalFailed > 0 {
		os.Exit(1)
	}
	return nil
}

func printGrade(grade *test.ScenarioGrade) {
	fmt.Printf("  %s (%d/%d)\n", grade.Scenario, grade.Passed, grade.Total)
	for _, r := range grade.Results {
		status := "✓"
		if !r.Pass {
			status = "✗"
		}
		if r.Reason != "" {
			fmt.Printf("    %s %s — %s\n", status, r.Criterion, r.Reason)
		} else {
			fmt.Printf("    %s %s\n", status, r.Criterion)
		}
	}
	fmt.Println()
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// newOllamaTextGenerator creates a TextGenerator that calls Ollama's /api/generate endpoint.
func newOllamaTextGenerator(baseURL, model string) test.TextGenerator {
	return func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		httpClient := &http.Client{Timeout: 60 * time.Second}

		reqBody := map[string]any{
			"model":  model,
			"prompt": prompt,
			"stream": false,
			"options": map[string]any{
				"temperature": temperature,
				"num_predict": maxTokens,
			},
		}

		data, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("marshal request: %w", err)
		}

		resp, err := httpClient.Post(baseURL+"/api/generate", "application/json", bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("ollama request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Response string `json:"response"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}

		return result.Response, nil
	}
}
