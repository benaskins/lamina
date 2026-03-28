package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LLM configures the optional LLM integration for doc review.
type LLM struct {
	Provider  string `yaml:"provider"`    // "anthropic", "openai"
	BaseURL   string `yaml:"base_url"`   // base URL for openai-compatible providers (e.g. http://localhost:8080/v1)
	Model     string `yaml:"model"`       // model name
	APIKeyEnv string `yaml:"api_key_env"` // env var holding the API key
}

// APIKey reads the API key from the configured environment variable.
func (l *LLM) APIKey() (string, error) {
	key := os.Getenv(l.APIKeyEnv)
	if key == "" {
		return "", fmt.Errorf("environment variable %s is not set", l.APIKeyEnv)
	}
	return key, nil
}

// Config holds lamina workspace configuration.
type Config struct {
	LLM     *LLM    `yaml:"llm,omitempty"`
	Release *Release `yaml:"release,omitempty"`
}

// Release configures release behaviour.
type Release struct {
	NotesProvider  string `yaml:"notes_provider,omitempty"`   // LLM provider for release notes (default: anthropic)
	NotesModel     string `yaml:"notes_model,omitempty"`      // model for release notes (default: claude-haiku-4-5-20251001)
	NotesAPIKeyEnv string `yaml:"notes_api_key_env,omitempty"` // env var for API key (default: same as llm.api_key_env)
	NotesBaseURL   string `yaml:"notes_base_url,omitempty"`   // base URL for openai-compatible provider
}

// NotesProvider returns the configured provider or the default.
func (c *Config) NotesProvider() string {
	if c.Release != nil && c.Release.NotesProvider != "" {
		return c.Release.NotesProvider
	}
	if c.LLM != nil && c.LLM.Provider != "" {
		return c.LLM.Provider
	}
	return "anthropic"
}

// NotesModel returns the configured model or the default.
func (c *Config) NotesModel() string {
	if c.Release != nil && c.Release.NotesModel != "" {
		return c.Release.NotesModel
	}
	return "claude-haiku-4-5-20251001"
}

// NotesBaseURL returns the base URL for the release notes provider.
func (c *Config) NotesBaseURL() string {
	if c.Release != nil && c.Release.NotesBaseURL != "" {
		return c.Release.NotesBaseURL
	}
	if c.LLM != nil && c.LLM.BaseURL != "" {
		return c.LLM.BaseURL
	}
	return ""
}

// NotesAPIKey returns the API key for the release notes provider.
func (c *Config) NotesAPIKey() (string, error) {
	envVar := ""
	if c.Release != nil && c.Release.NotesAPIKeyEnv != "" {
		envVar = c.Release.NotesAPIKeyEnv
	} else if c.LLM != nil && c.LLM.APIKeyEnv != "" {
		envVar = c.LLM.APIKeyEnv
	} else {
		envVar = "ANTHROPIC_API_KEY"
	}
	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf("environment variable %s is not set", envVar)
	}
	return key, nil
}

// LLMConfigured returns true if all required LLM fields are set.
func (c *Config) LLMConfigured() bool {
	return c.LLM != nil && c.LLM.Provider != "" && c.LLM.Model != "" && c.LLM.APIKeyEnv != ""
}

// DefaultPath returns the default config file path: ~/.config/lamina/config.yaml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "lamina", "config.yaml")
}

// Load reads a YAML config file. Returns an empty Config if the file doesn't exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
