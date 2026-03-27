package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"noto/internal/config"
)

const defaultSystemPrompt = `You are a helpful assistant. Respond clearly and concisely.`

// PromptStore manages prompt files for a profile.
type PromptStore struct {
	slug string
}

// NewPromptStore creates a PromptStore for the given profile slug.
func NewPromptStore(slug string) *PromptStore {
	return &PromptStore{slug: slug}
}

// GetSystemPrompt reads the profile's system prompt file.
// If the file does not exist, it initialises it with the default prompt.
func (ps *PromptStore) GetSystemPrompt() (string, error) {
	path, err := config.ProfileSystemPromptPath(ps.slug)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Initialise with default.
		if err2 := ps.SetSystemPrompt(defaultSystemPrompt); err2 != nil {
			return "", err2
		}
		return defaultSystemPrompt, nil
	}
	if err != nil {
		return "", fmt.Errorf("profile: read system prompt: %w", err)
	}
	return string(data), nil
}

// SetSystemPrompt writes content to the profile's system prompt file.
func (ps *PromptStore) SetSystemPrompt(content string) error {
	path, err := config.ProfileSystemPromptPath(ps.slug)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("profile: create prompts dir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

// PromptVersion returns a version string derived from the last modification time of the prompt file.
// Returns "default" if the file does not yet exist.
func (ps *PromptStore) PromptVersion() (string, error) {
	path, err := config.ProfileSystemPromptPath(ps.slug)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "default", nil
	}
	if err != nil {
		return "", fmt.Errorf("profile: stat prompt: %w", err)
	}
	return info.ModTime().UTC().Format(time.RFC3339Nano), nil
}
