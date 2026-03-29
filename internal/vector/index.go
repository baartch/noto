package vector

import (
	"errors"
)

// ErrIndexNotFound is returned when the vector index file does not exist for a profile.
var ErrIndexNotFound = errors.New("vector: index file not found")

// ErrIndexCorrupted is returned when the vector index file cannot be read or parsed.
var ErrIndexCorrupted = errors.New("vector: index file is corrupted")

// SourceType indicates the type of SQLite source entity that produced a vector entry.
type SourceType string

// Known vector source types.
const (
	SourceMemoryNote     SourceType = "memory_note"
	SourceSessionSummary SourceType = "session_summary"
	SourceMessage        SourceType = "message"
)

// Entry represents a single record tracked in the vector index.
type Entry struct {
	// ID is the unique identifier for this index entry.
	ID string

	// ProfileID scopes this entry to a specific profile.
	ProfileID string

	// SourceType indicates which SQLite table the source record lives in.
	SourceType SourceType

	// SourceID is the primary key of the source SQLite record.
	SourceID string

	// ChunkHash is a content hash to detect source changes requiring re-embedding.
	ChunkHash string

	// EmbeddingModel identifies the model used to produce the embedding.
	EmbeddingModel string

	// EmbeddingDim is the dimensionality of the embedding vector.
	EmbeddingDim int

	// Vector is the raw embedding slice.
	Vector []float32

	// VectorRef is an opaque reference to the stored vector (file offset, key, etc.).
	VectorRef string
}

// SearchResult is a single result from a vector top-k search.
type SearchResult struct {
	// Entry is the matched vector index entry.
	Entry Entry

	// Score is the similarity score (higher is more similar; range depends on metric).
	Score float32
}

// Index is the interface that all vector index adapters must satisfy.
// Implementations are expected to be per-profile and single-file.
type Index interface {
	// Upsert inserts or updates a vector entry. Identified by (SourceType, SourceID, ChunkHash).
	Upsert(entry Entry) error

	// Delete removes the vector entry for the given source.
	Delete(sourceType SourceType, sourceID string) error

	// Search returns the top-k most similar entries to the query vector.
	Search(query []float32, k int) ([]SearchResult, error)

	// Rebuild discards all existing index data and re-indexes from the provided entries.
	Rebuild(entries []Entry) error

	// Flush persists any in-memory state to disk.
	Flush() error

	// Close releases all resources held by the index.
	Close() error
}

// ---- no-op / stub implementation for build wiring --------------------------

// NoopIndex is a placeholder index that satisfies the Index interface but does nothing.
// It is used during initial wiring before a real index implementation is available.
type NoopIndex struct{}

// Upsert is a no-op implementation.
func (NoopIndex) Upsert(_ Entry) error { return nil }

// Delete is a no-op implementation.
func (NoopIndex) Delete(_ SourceType, _ string) error { return nil }

// Search is a no-op implementation.
func (NoopIndex) Search(_ []float32, _ int) ([]SearchResult, error) { return nil, nil }

// Rebuild is a no-op implementation.
func (NoopIndex) Rebuild(_ []Entry) error { return nil }

// Flush is a no-op implementation.
func (NoopIndex) Flush() error { return nil }

// Close is a no-op implementation.
func (NoopIndex) Close() error { return nil }

// ManifestStatus describes the health of a profile's vector index manifest.
type ManifestStatus string

// Known manifest status values.
const (
	ManifestReady      ManifestStatus = "ready"
	ManifestStale      ManifestStatus = "stale"
	ManifestRebuilding ManifestStatus = "rebuilding"
	ManifestFailed     ManifestStatus = "failed"
)

// Manifest holds per-profile vector index metadata.
type Manifest struct {
	ID                 string
	ProfileID          string
	IndexPath          string
	IndexFormatVersion string
	EmbeddingModel     string
	EmbeddingDim       int
	SourceStateVersion string
	Status             ManifestStatus
}

// ValidateManifest checks that a manifest's required fields are present.
func ValidateManifest(m *Manifest) error {
	if m.ProfileID == "" {
		return errors.New("vector: manifest missing profile_id")
	}
	if m.IndexPath == "" {
		return errors.New("vector: manifest missing index_path")
	}
	return nil
}
