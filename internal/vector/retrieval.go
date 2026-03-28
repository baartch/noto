package vector

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	warnFn    func(error)
}

// RetrievalOption configures the retrieval behavior.
type RetrievalOption func(*HybridRetrieval)

// WithWarnFunc registers a warning hook (used for missing/corrupt index).
func WithWarnFunc(fn func(error)) RetrievalOption {
	return func(r *HybridRetrieval) {
		r.warnFn = fn
	}
}

// NewHybridRetrieval creates a HybridRetrieval.
func NewHybridRetrieval(
	index Index,
	notes NoteLister,
	profileID string,
	opts ...RetrievalOption,
) *HybridRetrieval {
	r := &HybridRetrieval{
		index:     index,
		notes:     notes,
		profileID: profileID,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Retrieve performs a semantic search and hydrates results from the note lister.
// Falls back to top-N notes if the vector layer is unavailable.
func (r *HybridRetrieval) Retrieve(ctx context.Context, queryVector []float32, k int) ([]*HybridResult, error) {
	queryVector = normalize(queryVector)
	results, err := r.index.Search(queryVector, k)
	if err != nil {
		if r.warnFn != nil && (errors.Is(err, ErrIndexNotFound) || errors.Is(err, ErrIndexCorrupted)) {
			r.warnFn(err)
		}
		return r.fallback(ctx, k)
	}
	if len(results) == 0 {
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

func normalize(vec []float32) []float32 {
	if len(vec) == 0 {
		return vec
	}
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return vec
	}
	inv := float32(1 / math.Sqrt(sum))
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = v * inv
	}
	return out
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
