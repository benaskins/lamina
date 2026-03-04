package photo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	tool "github.com/benaskins/axon-tool"
)

// ImageGenConfig holds configuration for image generation baseline rules.
type ImageGenConfig struct {
	BaselinePrompt        string `json:"baseline_prompt"`
	PrivateBaselinePrompt string `json:"private_baseline_prompt"`
	MergeModel            string `json:"merge_model"`
	MergeInstruction      string `json:"merge_instruction"`
	PrivateMergeInstr     string `json:"private_merge_instruction"`
}

// LoadImageGenConfig loads the image generation config from a JSON file.
func LoadImageGenConfig(path string) (*ImageGenConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config ImageGenConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &config, nil
}

// PromptMerger intelligently merges baseline rules with agent context and scene prompts.
type PromptMerger struct {
	generate tool.TextGenerator
	config   *ImageGenConfig
}

// NewPromptMerger creates a new prompt merger.
func NewPromptMerger(generate tool.TextGenerator, config *ImageGenConfig) *PromptMerger {
	return &PromptMerger{
		generate: generate,
		config:   config,
	}
}

// MergePrompt intelligently merges baseline, agent system prompt, recent conversation,
// and scene into a single image generation prompt.
func (m *PromptMerger) MergePrompt(systemPrompt string, recentMessages []Message, scene string) (string, error) {
	return m.mergePromptWith(m.config.MergeInstruction, m.config.BaselinePrompt, systemPrompt, recentMessages, scene)
}

// MergePromptPrivate merges using the private instruction (no clothing cues).
func (m *PromptMerger) MergePromptPrivate(systemPrompt string, recentMessages []Message, scene string) (string, error) {
	instruction := m.config.PrivateMergeInstr
	if instruction == "" {
		instruction = m.config.MergeInstruction
	}
	baseline := m.config.PrivateBaselinePrompt
	if baseline == "" {
		baseline = m.config.BaselinePrompt
	}
	return m.mergePromptWith(instruction, baseline, systemPrompt, recentMessages, scene)
}

func (m *PromptMerger) mergePromptWith(mergeInstruction, baseline, systemPrompt string, recentMessages []Message, scene string) (string, error) {
	if m.config == nil {
		return scene, fmt.Errorf("no config loaded")
	}

	start := time.Now()

	// Build conversation context from recent messages
	var convParts []string
	for _, msg := range recentMessages {
		if msg.Content != "" {
			convParts = append(convParts, msg.Role+": "+msg.Content)
		}
	}
	conversation := strings.Join(convParts, "\n")

	// Build merge instruction by replacing placeholders (single-pass to prevent injection)
	replacer := strings.NewReplacer(
		"{baseline}", baseline,
		"{system_prompt}", systemPrompt,
		"{conversation}", conversation,
		"{scene}", scene,
	)
	instruction := replacer.Replace(mergeInstruction)

	result, err := m.generate(context.Background(), instruction)
	if err != nil {
		return "", fmt.Errorf("merge LLM call failed: %w", err)
	}

	result = strings.TrimSpace(result)
	latency := time.Since(start).Milliseconds()

	slog.Info("merged image prompt",
		"prompt", result,
		"system_prompt_len", len(systemPrompt),
		"conversation_len", len(conversation),
		"scene", scene,
		"merge_latency_ms", latency)

	return result, nil
}
