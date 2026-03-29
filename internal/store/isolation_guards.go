package store

import (
	"context"
	"fmt"
)

// IsolationGuards provides cross-cutting profile isolation checks for all repositories.
type IsolationGuards struct {
	db *DB
}

// NewIsolationGuards creates a new IsolationGuards checker.
func NewIsolationGuards(db *DB) *IsolationGuards {
	return &IsolationGuards{db: db}
}

// AssertNoteOwnership verifies that a memory note belongs to the given profile.
func (g *IsolationGuards) AssertNoteOwnership(ctx context.Context, profileID, noteID string) error {
	var count int
	err := g.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_notes WHERE id = ? AND profile_id = ?`,
		noteID, profileID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("store: isolation check note: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("store: note %q does not belong to profile %q", noteID, profileID)
	}
	return nil
}

// AssertConversationOwnership verifies that a conversation belongs to the given profile.
func (g *IsolationGuards) AssertConversationOwnership(ctx context.Context, profileID, conversationID string) error {
	var count int
	err := g.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM conversations WHERE id = ? AND profile_id = ?`,
		conversationID, profileID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("store: isolation check conversation: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("store: conversation %q does not belong to profile %q", conversationID, profileID)
	}
	return nil
}

// AssertCacheOwnership verifies that a cache entry belongs to the given profile.
func (g *IsolationGuards) AssertCacheOwnership(ctx context.Context, profileID, cacheKey string) error {
	var count int
	err := g.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM context_cache WHERE cache_key = ? AND profile_id = ?`,
		cacheKey, profileID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("store: isolation check cache: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("store: cache key %q does not belong to profile %q", cacheKey, profileID)
	}
	return nil
}
