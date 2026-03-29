package vector

import (
	"context"
	"fmt"
)

// ManifestStatusSetter is the interface for updating the vector manifest status.
// It accepts a string to avoid type coupling with the store package.
type ManifestStatusSetter interface {
	SetManifestStatusStr(ctx context.Context, profileID string, status string) error
}

// Rebuilder rebuilds the vector index from a set of notes.
type Rebuilder struct {
	manifestSetter ManifestStatusSetter
	index          Index
	profileID      string
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

// Rebuild discards the existing index and re-indexes the given notes.
func (rb *Rebuilder) Rebuild(ctx context.Context, notes []MemoryNoteRecord) error {
	// Mark manifest as rebuilding.
	_ = rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestRebuilding))

	entries := make([]Entry, 0, len(notes))
	for _, note := range notes {
		entries = append(entries, Entry{
			ID:             "ve-" + note.ID,
			ProfileID:      rb.profileID,
			SourceType:     SourceMemoryNote,
			SourceID:       note.ID,
			ChunkHash:      ContentHash(note.Content),
			EmbeddingModel: "",
		})
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
