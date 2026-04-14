package vector

import (
	"context"

	"noto/internal/provider"
)

const defaultDedupThreshold = 0.92

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

// VectorDeduper implements Deduper using the vector index.
type VectorDeduper struct {
	index     Index
	embedder  Embedder
	model     string
	threshold float32
	profileID string
	warnFn    func(error)
}

// NewVectorDeduper constructs a VectorDeduper.
func NewVectorDeduper(index Index, profileID string, embedder Embedder, model string) *VectorDeduper {
	return &VectorDeduper{index: index, embedder: embedder, model: model, threshold: defaultDedupThreshold, profileID: profileID}
}

// WithWarnFunc sets a warning handler.
func (d *VectorDeduper) WithWarnFunc(fn func(error)) *VectorDeduper {
	d.warnFn = fn
	return d
}

// WithThreshold sets the dedup similarity threshold.
func (d *VectorDeduper) WithThreshold(threshold float32) *VectorDeduper {
	d.threshold = threshold
	return d
}

// CheckDuplicate compares the candidate against existing vectors.
func (d *VectorDeduper) CheckDuplicate(ctx context.Context, profileID string, content string) (DedupResult, error) {
	if d == nil || d.index == nil || d.embedder == nil {
		return DedupResult{IsDuplicate: false}, nil
	}
	if profileID == "" {
		profileID = d.profileID
	}
	if fileIndex, ok := d.index.(*FileIndex); ok {
		if profileID != "" {
			fileIndex.WithProfile(profileID)
		}
		if err := fileIndex.Load(); err != nil {
			if d.warnFn != nil && (err == ErrIndexNotFound || err == ErrIndexCorrupted) {
				d.warnFn(err)
				return DedupResult{IsDuplicate: false}, nil
			}
			return DedupResult{IsDuplicate: false}, err
		}
	}
	resp, err := d.embedder.Embed(ctx, provider.EmbeddingRequest{Input: content, Model: d.model})
	if err != nil {
		return DedupResult{IsDuplicate: false}, err
	}
	results, err := d.index.Search(resp.Embedding, 3)
	if err != nil {
		if d.warnFn != nil && (err == ErrIndexNotFound || err == ErrIndexCorrupted) {
			d.warnFn(err)
			return DedupResult{IsDuplicate: false}, nil
		}
		return DedupResult{IsDuplicate: false}, err
	}
	best := DedupResult{IsDuplicate: false}
	for _, res := range results {
		if res.Entry.SourceType != SourceMemoryNote {
			continue
		}
		if res.Score > best.Score {
			best.Score = res.Score
			best.MatchID = res.Entry.SourceID
		}
	}
	if best.MatchID != "" && best.Score >= d.threshold {
		best.IsDuplicate = true
	}
	return best, nil
}
