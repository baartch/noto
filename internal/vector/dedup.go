package vector

import "context"

// DedupResult captures duplicate detection results.
type DedupResult struct {
	IsDuplicate bool
	MatchID     string
	Score       float32
}

// Deduper checks candidate notes against the vector index.
type Deduper interface {
	CheckDuplicate(ctx context.Context, profileID string, content string) (DedupResult, error)
}
