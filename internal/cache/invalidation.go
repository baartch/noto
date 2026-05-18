package cache

import (
	"context"
	"fmt"

	"noto/internal/store"
)

// InvalidationTriggers handles cache invalidation when profile data changes.
type InvalidationTriggers struct {
	cacheRepo *store.ContextCacheRepo
}

// NewInvalidationTriggers creates an InvalidationTriggers.
func NewInvalidationTriggers(cacheRepo *store.ContextCacheRepo) *InvalidationTriggers {
	return &InvalidationTriggers{cacheRepo: cacheRepo}
}

// OnPromptChange invalidates all cache entries for a profile when the system prompt changes.
func (t *InvalidationTriggers) OnPromptChange(ctx context.Context, profileID string) error {
	if err := t.cacheRepo.InvalidateAll(ctx, profileID); err != nil {
		return fmt.Errorf("cache: invalidate on prompt change: %w", err)
	}
	return nil
}

// OnMemoryChange invalidates the context cache for a profile when memory notes change.
func (t *InvalidationTriggers) OnMemoryChange(ctx context.Context, profileID string) error {
	if err := t.cacheRepo.InvalidateAll(ctx, profileID); err != nil {
		return fmt.Errorf("cache: invalidate on memory change: %w", err)
	}
	return nil
}

// OnTokenBudgetChange invalidates cache entries when token budget changes.
func (t *InvalidationTriggers) OnTokenBudgetChange(ctx context.Context, profileID string) error {
	if err := t.cacheRepo.InvalidateAll(ctx, profileID); err != nil {
		return fmt.Errorf("cache: invalidate on token budget change: %w", err)
	}
	return nil
}

// OnEmbeddingModelChange invalidates cache entries when embedding model changes.
func (t *InvalidationTriggers) OnEmbeddingModelChange(ctx context.Context, profileID string) error {
	if err := t.cacheRepo.InvalidateAll(ctx, profileID); err != nil {
		return fmt.Errorf("cache: invalidate on embedding model change: %w", err)
	}
	return nil
}

// OnNoteCreated invalidates cache entries when a note is created.
func (t *InvalidationTriggers) OnNoteCreated(ctx context.Context, profileID string) error {
	return t.OnMemoryChange(ctx, profileID)
}

// OnNoteUpdated invalidates cache entries when a note is updated.
func (t *InvalidationTriggers) OnNoteUpdated(ctx context.Context, profileID string) error {
	return t.OnMemoryChange(ctx, profileID)
}

// OnNoteDeleted invalidates cache entries when a note is deleted.
func (t *InvalidationTriggers) OnNoteDeleted(ctx context.Context, profileID string) error {
	return t.OnMemoryChange(ctx, profileID)
}
