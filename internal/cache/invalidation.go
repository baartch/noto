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
