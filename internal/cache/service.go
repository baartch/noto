package cache

import (
	"context"
	"fmt"
	"time"

	"noto/internal/store"
)

const defaultCacheTTL = 24 * time.Hour

// Service manages context cache lifecycle: build, load, invalidate.
type Service struct {
	repo *store.ContextCacheRepo
}

// NewService creates a cache Service.
func NewService(repo *store.ContextCacheRepo) *Service {
	return &Service{repo: repo}
}

// Get retrieves a valid (non-expired) cache entry for the given profile and key.
// Returns nil, nil if no valid entry exists.
func (s *Service) Get(ctx context.Context, profileID, cacheKey string) (*store.ContextCacheEntry, error) {
	entry, err := s.repo.Get(ctx, profileID, cacheKey)
	if err != nil {
		return nil, nil // treat miss as nil, nil
	}
	if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
		_ = s.repo.Invalidate(ctx, profileID, cacheKey)
		return nil, nil
	}
	return entry, nil
}

// Put stores a context cache entry with the default TTL.
func (s *Service) Put(ctx context.Context, profileID, cacheKey, payload, sourceNoteIDs, promptVersion, stateVersion string) error {
	expiresAt := time.Now().Add(defaultCacheTTL)
	entry := &store.ContextCacheEntry{
		ID:            fmt.Sprintf("cc-%x", time.Now().UnixNano()),
		ProfileID:     profileID,
		CacheKey:      cacheKey,
		Payload:       payload,
		SourceNoteIDs: sourceNoteIDs,
		PromptVersion: promptVersion,
		StateVersion:  stateVersion,
		CreatedAt:     time.Now().UTC(),
		ExpiresAt:     &expiresAt,
	}
	return s.repo.Upsert(ctx, entry)
}

// Invalidate removes a specific cache entry.
func (s *Service) Invalidate(ctx context.Context, profileID, cacheKey string) error {
	return s.repo.Invalidate(ctx, profileID, cacheKey)
}

// InvalidateAll removes all cache entries for a profile.
func (s *Service) InvalidateAll(ctx context.Context, profileID string) error {
	return s.repo.InvalidateAll(ctx, profileID)
}
