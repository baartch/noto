package vector

import (
	"context"
	"fmt"
)

// NoteGetter fetches a single memory note by its ID.
type NoteGetter interface {
	GetByID(ctx context.Context, id string) (*MemoryNoteRecord, error)
}

// NoteLister lists all notes for a profile.
type NoteLister interface {
	ListByProfile(ctx context.Context, profileID string) ([]MemoryNoteRecord, error)
}

// HybridResult is a single result from hybrid retrieval.
type HybridResult struct {
	Note  MemoryNoteRecord
	Score float32
}

// HybridRetrieval orchestrates vector candidate recall with SQLite authoritative hydration.
type HybridRetrieval struct {
	index     Index
	notes     NoteLister
	profileID string
}

// NewHybridRetrieval creates a HybridRetrieval.
func NewHybridRetrieval(
	index Index,
	notes NoteLister,
	profileID string,
) *HybridRetrieval {
	return &HybridRetrieval{
		index:     index,
		notes:     notes,
		profileID: profileID,
	}
}

// Retrieve performs a semantic search and hydrates results from the note lister.
// Falls back to top-N notes if the vector layer is unavailable.
func (r *HybridRetrieval) Retrieve(ctx context.Context, queryVector []float32, k int) ([]*HybridResult, error) {
	results, err := r.index.Search(queryVector, k)
	if err != nil || len(results) == 0 {
		return r.fallback(ctx, k)
	}

	hydrated := make([]*HybridResult, 0, len(results))
	for _, sr := range results {
		if sr.Entry.SourceType != SourceMemoryNote {
			continue
		}
		notes, err := r.notes.ListByProfile(ctx, r.profileID)
		if err != nil {
			continue
		}
		for _, n := range notes {
			if n.ID == sr.Entry.SourceID {
				hydrated = append(hydrated, &HybridResult{Note: n, Score: sr.Score})
				break
			}
		}
	}

	if len(hydrated) == 0 {
		return r.fallback(ctx, k)
	}
	return hydrated, nil
}

func (r *HybridRetrieval) fallback(ctx context.Context, k int) ([]*HybridResult, error) {
	notes, err := r.notes.ListByProfile(ctx, r.profileID)
	if err != nil {
		return nil, fmt.Errorf("vector: fallback list notes: %w", err)
	}
	if len(notes) > k {
		notes = notes[:k]
	}
	out := make([]*HybridResult, 0, len(notes))
	for _, n := range notes {
		out = append(out, &HybridResult{Note: n, Score: 0})
	}
	return out, nil
}
