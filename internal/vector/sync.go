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

// ManifestEntry describes a persisted vector entry.
type ManifestEntry struct {
	ID             string
	ProfileID      string
	SourceType     SourceType
	SourceID       string
	ChunkHash      string
	EmbeddingModel string
	EmbeddingDim   int
	VectorRef      string
}

// ManifestEntryRepo persists manifest entries.
type ManifestEntryRepo interface {
	UpsertEntry(ctx context.Context, e *ManifestEntry) error
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
	manifest       ManifestEntryRepo
}

// NewSyncer creates a Syncer for the given profile.
func NewSyncer(index Index, profileID string, embedder Embedder, embeddingModel string) *Syncer {
	return &Syncer{index: index, profileID: profileID, embedder: embedder, embeddingModel: embeddingModel}
}

// WithManifest sets the manifest entry upserter.
func (s *Syncer) WithManifest(manifest ManifestEntryRepo) *Syncer {
	s.manifest = manifest
	return s
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
		if fileIndex, ok := s.index.(*FileIndex); ok {
			if ref, ok := fileIndex.VectorRefFor(SourceMemoryNote, note.ID); ok {
				entry.VectorRef = ref
			}
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
		if fileIndex, ok := s.index.(*FileIndex); ok {
			if ref, ok := fileIndex.VectorRefFor(SourceMemoryNote, note.ID); ok {
				entry.VectorRef = ref
			}
		}
		if s.manifest != nil {
			_ = s.manifest.UpsertEntry(ctx, &ManifestEntry{
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
	return s.index.Flush()
}

// ContentHash returns a hex SHA-256 hash of the given content string.
func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
