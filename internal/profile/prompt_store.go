package profile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"noto/internal/store"
)

const defaultSystemPrompt = "You are Noto. A buddy who takes notes."

// PromptStore manages reading and writing the profile system prompt from SQLite.
type PromptStore struct {
	profileID string
	repo      *store.SystemPromptRepo
}

// NewPromptStore creates a PromptStore.
func NewPromptStore(profileID string, repo *store.SystemPromptRepo) *PromptStore {
	return &PromptStore{profileID: profileID, repo: repo}
}

// GetSystemPrompt returns the stored prompt or a default if missing.
func (ps *PromptStore) GetSystemPrompt(ctx context.Context) (string, error) {
	if ps.repo == nil {
		return defaultSystemPrompt, nil
	}
	prompt, err := ps.repo.GetByProfile(ctx, ps.profileID)
	if err != nil {
		if errors.Is(err, store.ErrSystemPromptNotFound) {
			return defaultSystemPrompt, nil
		}
		return "", fmt.Errorf("profile: get system prompt: %w", err)
	}
	if prompt.Prompt == "" {
		return defaultSystemPrompt, nil
	}
	return prompt.Prompt, nil
}

// SetSystemPrompt writes the prompt to SQLite.
func (ps *PromptStore) SetSystemPrompt(ctx context.Context, content string) error {
	if ps.repo == nil {
		return errors.New("profile: system prompt repo is nil")
	}
	p := &store.SystemPrompt{
		ID:        fmt.Sprintf("sp-%x", time.Now().UnixNano()),
		ProfileID: ps.profileID,
		Prompt:    content,
	}
	if err := ps.repo.Upsert(ctx, p); err != nil {
		return fmt.Errorf("profile: set system prompt: %w", err)
	}
	return nil
}
