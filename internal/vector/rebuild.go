package vector

import (
	"context"
	"errors"
	"fmt"

	"noto/internal/provider"
)

// ManifestStatusSetter is the interface for updating the vector manifest status.
// It accepts a string to avoid type coupling with the store package.
type ManifestStatusSetter interface {
	SetManifestStatusStr(ctx context.Context, profileID string, status string) error
}

// Rebuilder rebuilds the vector index from a set of notes.
type Rebuilder struct {
	manifestSetter ManifestStatusSetter
	manifestRepo   ManifestEntryRepo
	index          Index
	profileID      string
	embedder       Embedder
	embeddingModel string
}

// NewRebuilder creates a Rebuilder.
func NewRebuilder(
	manifestSetter ManifestStatusSetter,
	index Index,
	profileID string,
) *Rebuilder {
	return &Rebuilder{
		manifestSetter: manifestSetter,
		index:          index,
		profileID:      profileID,
	}
}

// WithManifest sets the manifest repo for rebuilt entries.
func (rb *Rebuilder) WithManifest(repo ManifestEntryRepo) *Rebuilder {
	rb.manifestRepo = repo
	return rb
}

// WithEmbedder sets the embedder and model for rebuild.
func (rb *Rebuilder) WithEmbedder(embedder Embedder, model string) *Rebuilder {
	rb.embedder = embedder
	rb.embeddingModel = model
	return rb
}

// Rebuild discards the existing index and re-indexes the given notes.
func (rb *Rebuilder) Rebuild(ctx context.Context, notes []MemoryNoteRecord) error {
	// Mark manifest as rebuilding.
	_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestRebuilding))

	if rb.embedder == nil {
		return errors.New("vector: embedder not configured")
	}

	entries := make([]Entry, 0, len(notes))
	for _, note := range notes {
		resp, err := rb.embedder.Embed(ctx, provider.EmbeddingRequest{Input: note.Content, Model: rb.embeddingModel})
		if err != nil {
			_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestFailed))
			return fmt.Errorf("vector: embed note %s: %w", note.ID, err)
		}
		entry := Entry{
			ID:             "ve-" + note.ID,
			ProfileID:      rb.profileID,
			SourceType:     SourceMemoryNote,
			SourceID:       note.ID,
			ChunkHash:      ContentHash(note.Content),
			EmbeddingModel: rb.embeddingModel,
			EmbeddingDim:   len(resp.Embedding),
			Vector:         resp.Embedding,
		}
		entries = append(entries, entry)
		if rb.manifestRepo != nil {
			_ = rb.manifestRepo.UpsertEntry(ctx, &ManifestEntry{
				ID:             entry.ID,
				ProfileID:      entry.ProfileID,
				SourceType:     entry.SourceType,
				SourceID:       entry.SourceID,
				ChunkHash:      entry.ChunkHash,
				EmbeddingModel: entry.EmbeddingModel,
				EmbeddingDim:   entry.EmbeddingDim,
				VectorRef:      entry.VectorRef,
			})
		}
	}

	if err := rb.index.Rebuild(entries); err != nil {
		_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestFailed))
		return fmt.Errorf("vector: rebuild index: %w", err)
	}
	if err := rb.index.Flush(); err != nil {
		_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestFailed))
		return fmt.Errorf("vector: flush after rebuild: %w", err)
	}

	_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestReady))
	return nil
}
