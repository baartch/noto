package vector

import (
	"context"
	"errors"
	"fmt"

	"noto/internal/provider"
	"noto/internal/startup"
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
	progress       startup.Reporter
}

// WithProgress configures startup progress reporting.
func (rb *Rebuilder) WithProgress(report startup.Reporter) *Rebuilder {
	rb.progress = report
	return rb
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
func (rb *Rebuilder) Rebuild(ctx context.Context, notes []MemoryNoteRecord) (resultErr error) {
	event := startup.Event{Phase: "rebuild", State: "start", Total: len(notes)}
	rb.progress.Send(event)
	defer func() {
		event.State = "success"
		if resultErr != nil {
			event.State = "failure"
		}
		rb.progress.Send(event)
	}()
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
		event.Done++
		event.State = "update"
		rb.progress.Send(event)
	}

	if err := rb.index.Rebuild(entries); err != nil {
		_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestFailed))
		return fmt.Errorf("vector: rebuild index: %w", err)
	}
	if err := rb.index.Flush(); err != nil {
		_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestFailed))
		return fmt.Errorf("vector: flush after rebuild: %w", err)
	}
	if rb.manifestRepo != nil {
		for _, entry := range entries {
			if index, ok := rb.index.(*FileIndex); ok {
				entry.VectorRef, _ = index.VectorRefFor(entry.SourceType, entry.SourceID)
			}
			if err := rb.manifestRepo.UpsertEntry(ctx, &ManifestEntry{
				ID: entry.ID, ProfileID: entry.ProfileID, SourceType: entry.SourceType,
				SourceID: entry.SourceID, ChunkHash: entry.ChunkHash,
				EmbeddingModel: entry.EmbeddingModel, EmbeddingDim: entry.EmbeddingDim, VectorRef: entry.VectorRef,
			}); err != nil {
				_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestFailed))
				return fmt.Errorf("vector: update manifest for %s: %w", entry.SourceID, err)
			}
		}
	}

	_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestReady))
	return nil
}
