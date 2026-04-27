package profile

import (
	"context"
	"fmt"

	"noto/internal/config"
)

const defaultSystemPrompt = "You are Noto. A buddy who takes notes."

// PromptStore manages reading and writing profile prompts from profile-local markdown files.
type PromptStore struct {
	profileSlug string
}

// NewPromptStore creates a PromptStore. The second parameter is kept for backward compatibility
// with legacy call sites that previously passed a DB repo.
func NewPromptStore(profileSlug string, _ any) *PromptStore {
	return &PromptStore{profileSlug: profileSlug}
}

// GetSystemPrompt returns the file-backed prompt, bootstrapping defaults if missing.
func (ps *PromptStore) GetSystemPrompt(ctx context.Context) (string, error) {
	_ = ctx
	content, _, err := config.ReadSystemPromptFile(ps.profileSlug)
	if err != nil {
		return "", fmt.Errorf("profile: get system prompt: %w", err)
	}
	if content == "" {
		return defaultSystemPrompt, nil
	}
	return content, nil
}

// SetSystemPrompt writes the prompt to the profile prompt markdown file.
func (ps *PromptStore) SetSystemPrompt(ctx context.Context, content string) error {
	_ = ctx
	if err := config.WriteSystemPromptFile(ps.profileSlug, content); err != nil {
		return fmt.Errorf("profile: set system prompt: %w", err)
	}
	return nil
}
