package main

import (
	"fmt"

	talk "github.com/benaskins/axon-talk"
	"github.com/benaskins/axon-talk/anthropic"
	"github.com/benaskins/axon-talk/openai"
)

func newLLMClient(provider, baseURL, apiKey string) (talk.LLMClient, error) {
	switch provider {
	case "anthropic":
		return anthropic.NewClient("https://api.anthropic.com", apiKey), nil
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return openai.NewClient(baseURL, apiKey), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q (supported: anthropic, openai)", provider)
	}
}
