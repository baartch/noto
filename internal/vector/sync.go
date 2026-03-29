package vector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"noto/internal/provider"
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

// Embedder generates embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, req provider.EmbeddingRequest) (*provider.EmbeddingResponse, error)
}

// Syncer synchronizes SQLite memory sources into the vector index.
type Syncer struct {
	index          Index
	profileID      string
	embeddingModel string
	embedder       Embedder
}

// NewSyncer creates a Syncer for the given profile.
func NewSyncer(index Index, profileID string, embedder Embedder, embeddingModel string) *Syncer {
	return &Syncer{index: index, profileID: profileID, embedder: embedder, embeddingModel: embeddingModel}
}

// SyncNotes upserts the given notes into the vector index.
func (s *Syncer) SyncNotes(ctx context.Context, notes []MemoryNoteRecord) error {
	for _, note := range notes {
		chunkHash := ContentHash(note.Content)
		entry := Entry{
			ID:             "ve-" + note.ID,
			ProfileID:      s.profileID,
			SourceType:     SourceMemoryNote,
			SourceID:       note.ID,
			ChunkHash:      chunkHash,
			EmbeddingModel: s.embeddingModel,
		}
		if s.embedder == nil {
			return errors.New("vector: embedder not configured")
		}
		resp, err := s.embedder.Embed(ctx, provider.EmbeddingRequest{Input: note.Content, Model: s.embeddingModel})
		if err != nil {
			return fmt.Errorf("vector: embed note %s: %w", note.ID, err)
		}
		entry.Vector = resp.Embedding
		entry.EmbeddingDim = len(resp.Embedding)
		if err := s.index.Upsert(entry); err != nil {
			return fmt.Errorf("vector: upsert index entry %s: %w", note.ID, err)
		}
	}
	return s.index.Flush()
}

// ContentHash returns a hex SHA-256 hash of the given content string.
func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
