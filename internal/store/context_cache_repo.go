package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrCacheNotFound is returned when a context cache entry is not found.
var ErrCacheNotFound = errors.New("store: context cache entry not found")

// ContextCacheEntry is the data model for a reusable assembled context artifact.
type ContextCacheEntry struct {
	ID            string
	ProfileID     string
	CacheKey      string
	Payload       string
	SourceNoteIDs string // JSON array
	PromptVersion string
	StateVersion  string
	CreatedAt     time.Time
	ExpiresAt     *time.Time
}

// ContextCacheRepo manages CRUD operations for context cache entries.
type ContextCacheRepo struct {
	db *DB
}

// NewContextCacheRepo creates a new ContextCacheRepo.
func NewContextCacheRepo(db *DB) *ContextCacheRepo {
	return &ContextCacheRepo{db: db}
}

// Get retrieves a context cache entry by profile and cache key.
func (r *ContextCacheRepo) Get(ctx context.Context, profileID, cacheKey string) (*ContextCacheEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, cache_key, payload, source_note_ids,
		       prompt_version, state_version, created_at, expires_at
		FROM context_cache
		WHERE profile_id = ? AND cache_key = ?
	`, profileID, cacheKey)

	e := &ContextCacheEntry{}
	err := row.Scan(&e.ID, &e.ProfileID, &e.CacheKey, &e.Payload, &e.SourceNoteIDs,
		&e.PromptVersion, &e.StateVersion, &e.CreatedAt, &e.ExpiresAt)
	if errors.Is(err, nil) {
		return e, nil
	}
	return nil, fmt.Errorf("store: get context cache: %w", err)
}

// Upsert inserts or replaces a context cache entry.
func (r *ContextCacheRepo) Upsert(ctx context.Context, e *ContextCacheEntry) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO context_cache
			(id, profile_id, cache_key, payload, source_note_ids,
			 prompt_version, state_version, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, cache_key) DO UPDATE SET
			payload        = excluded.payload,
			source_note_ids = excluded.source_note_ids,
			prompt_version = excluded.prompt_version,
			state_version  = excluded.state_version,
			created_at     = excluded.created_at,
			expires_at     = excluded.expires_at
	`, e.ID, e.ProfileID, e.CacheKey, e.Payload, e.SourceNoteIDs,
		e.PromptVersion, e.StateVersion, e.CreatedAt, e.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store: upsert context cache: %w", err)
	}
	return nil
}

// Invalidate removes a specific cache entry by profile and key.
func (r *ContextCacheRepo) Invalidate(ctx context.Context, profileID, cacheKey string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM context_cache WHERE profile_id = ? AND cache_key = ?
	`, profileID, cacheKey)
	return err
}

// InvalidateAll removes all cache entries for a profile.
func (r *ContextCacheRepo) InvalidateAll(ctx context.Context, profileID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM context_cache WHERE profile_id = ?
	`, profileID)
	return err
}

// PruneExpired removes all expired cache entries for a profile.
func (r *ContextCacheRepo) PruneExpired(ctx context.Context, profileID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM context_cache
		WHERE profile_id = ? AND expires_at IS NOT NULL AND expires_at < ?
	`, profileID, time.Now().UTC())
	return err
}
