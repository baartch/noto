package vector

import (
	"context"
	"crypto/sha256"
	"fmt"
)

// NoteProvider is the minimal interface the Syncer needs from the store.
type NoteProvider interface {
	ListByProfile(ctx context.Context, profileID string) ([]MemoryNoteRecord, error)
}

// MemoryNoteRecord is a minimal note representation used by the vector layer.
type MemoryNoteRecord struct {
	ID      string
	Content string
}

// ManifestEntryUpserter is the interface for persisting vector manifest entries.
type ManifestEntryUpserter interface {
	UpsertEntry(ctx context.Context, e *Entry) error
}

// Syncer synchronises SQLite memory sources into the vector index.
type Syncer struct {
	notes     []MemoryNoteRecord
	index     Index
	profileID string
}

// NewSyncer creates a Syncer for the given profile.
func NewSyncer(index Index, profileID string) *Syncer {
	return &Syncer{index: index, profileID: profileID}
}

// SyncNotes upserts the given notes into the vector index.
func (s *Syncer) SyncNotes(ctx context.Context, notes []MemoryNoteRecord) error {
	for _, note := range notes {
		chunkHash := ContentHash(note.Content)
		entry := Entry{
			ID:             fmt.Sprintf("ve-%s", note.ID),
			ProfileID:      s.profileID,
			SourceType:     SourceMemoryNote,
			SourceID:       note.ID,
			ChunkHash:      chunkHash,
			EmbeddingModel: "noop",
			EmbeddingDim:   0,
			Vector:         nil,
			VectorRef:      "",
		}
		if err := s.index.Upsert(entry); err != nil {
			return fmt.Errorf("vector: upsert index entry %s: %w", note.ID, err)
		}
	}
	return s.index.Flush()
}

// ContentHash returns a hex SHA-256 hash of the given content string.
func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}
