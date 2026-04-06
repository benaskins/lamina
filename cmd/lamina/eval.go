package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	talk "github.com/benaskins/axon-talk"
	"github.com/benaskins/axon-talk/openai"
	eval "github.com/benaskins/axon-eval"
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

	plan, err := eval.LoadPlan(planPath)
	if err != nil {
		return fmt.Errorf("load plan: %w", err)
	}

	cfg := eval.Config{
		AuthURL:      envOrDefault("AUTH_URL", "https://auth.hestia.internal"),
		ChatURL:      envOrDefault("CHAT_URL", "https://chat.hestia.internal"),
		AnalyticsURL: envOrDefault("ANALYTICS_URL", "https://analytics.hestia.internal"),
	}

	client, err := eval.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	// Set up optional LLM judge
	var judge eval.Judge
	judgeModel := os.Getenv("JUDGE_MODEL")
	if judgeModel != "" {
		baseURL := envOrDefault("JUDGE_BASE_URL", "http://localhost:8080/v1")
		token := os.Getenv("JUDGE_API_KEY")
		llm := openai.NewClient(baseURL, token)
		generate := newTextGenerator(llm, judgeModel)
		judge = eval.NewOllamaJudge(generate)
	}

	fmt.Printf("━━━ %s ━━━\n", plan.Name)
	if judge != nil {
		fmt.Printf("Judge model: %s\n", judgeModel)
	}
	fmt.Println()

	// Build scenarios from plan
	scenarios := make([]eval.Scenario, len(plan.Scenarios))
	for i, ps := range plan.Scenarios {
		scenarios[i] = eval.Conversation(ps.Name, []eval.Message{
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

		grade := eval.GradeScenario(cmd.Context(), ps, chatResult, judge)
		printGrade(grade)

		totalPassed += grade.Passed
		totalFailed += grade.Failed
	}

	// Summary
	fmt.Println("━━━ Summary ━━━")
	fmt.Printf("  Run ID: %s\n", run.ID)
	fmt.Printf("  Criteria: %d passed, %d failed\n", totalPassed, totalFailed)

	if totalFailed > 0 {
		return fmt.Errorf("%d eval criteria failed", totalFailed)
	}
	return nil
}

func printGrade(grade *eval.ScenarioGrade) {
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

// newTextGenerator wraps a talk.LLMClient into an eval.TextGenerator.
func newTextGenerator(client talk.LLMClient, model string) eval.TextGenerator {
	return func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		req := &talk.Request{
			Model: model,
			Messages: []talk.Message{
				{Role: "user", Content: prompt},
			},
			Options: map[string]any{
				"temperature": temperature,
				"max_tokens":  maxTokens,
			},
		}

		var buf strings.Builder
		err := client.Chat(ctx, req, func(resp talk.Response) error {
			buf.WriteString(resp.Content)
			return nil
		})
		if err != nil {
			return "", err
		}
		return buf.String(), nil
	}
}
