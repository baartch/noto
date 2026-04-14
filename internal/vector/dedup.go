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

// DeduperImpl implements Deduper using the vector index.
type DeduperImpl struct {
	index     Index
	embedder  Embedder
	model     string
	threshold float32
	profileID string
	warnFn    func(error)
}

// NewVectorDeduper constructs a DeduperImpl.
func NewVectorDeduper(index Index, profileID string, embedder Embedder, model string) *DeduperImpl {
	return &DeduperImpl{index: index, embedder: embedder, model: model, threshold: defaultDedupThreshold, profileID: profileID}
}

// WithWarnFunc sets a warning handler.
func (d *DeduperImpl) WithWarnFunc(fn func(error)) *DeduperImpl {
	d.warnFn = fn
	return d
}

// WithThreshold sets the dedup similarity threshold.
func (d *DeduperImpl) WithThreshold(threshold float32) *DeduperImpl {
	d.threshold = threshold
	return d
}

// CheckDuplicate compares the candidate against existing vectors.
func (d *DeduperImpl) CheckDuplicate(ctx context.Context, profileID string, content string) (DedupResult, error) {
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
