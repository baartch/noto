package vector

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
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

// FileIndex implements a single-file vector index using a codec + HNSW graph.
type FileIndex struct {
	path           string
	codec          vecfile.Codec
	graph          hnsw.Graph
	entries        map[string]Entry
	refToKey       map[string]string
	vectors        [][]float32
	embeddingModel string
	embeddingDim   int
	loaded         bool
	profileID      string
}

// NewFileIndex creates a file-backed vector index.
func NewFileIndex(path string, codec vecfile.Codec, graph hnsw.Graph) *FileIndex {
	return &FileIndex{
		path:     path,
		codec:    codec,
		graph:    graph,
		entries:  make(map[string]Entry),
		refToKey: make(map[string]string),
	}
}

// WithProfile sets the profile identifier to record in the header.
func (f *FileIndex) WithProfile(profileID string) {
	f.profileID = profileID
}

// Load reads memory.vec into memory if it exists.
func (f *FileIndex) Load() error {
	if f.codec == nil {
		return errors.New("vector: codec not configured")
	}
	file, err := os.Open(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrIndexNotFound
		}
		return fmt.Errorf("vector: open index: %w", err)
	}
	defer file.Close()

	header, err := f.codec.ReadHeader(file)
	if err != nil {
		return ErrIndexCorrupted
	}
	f.embeddingModel = header.EmbeddingModel
	f.embeddingDim = int(header.EmbeddingDim)

	vectors, err := f.codec.ReadVectors(file, int(header.EntryCount), int(header.EmbeddingDim))
	if err != nil {
		return ErrIndexCorrupted
	}
	f.vectors = reshapeVectors(vectors, int(header.EntryCount), int(header.EmbeddingDim))

	graphBytes, err := f.codec.ReadGraph(file)
	if err != nil {
		return ErrIndexCorrupted
	}
	if f.graph != nil && len(graphBytes) > 0 {
		if err := f.graph.Deserialize(graphBytes); err != nil {
			return ErrIndexCorrupted
		}
		for i, vec := range f.vectors {
			entry, ok := f.entries[f.refToKey[strconv.Itoa(i)]]
			if ok {
				_ = f.graph.Insert(entry.ID, vec)
			}
		}
	}
	f.loaded = true
	return nil
}

// Upsert inserts or updates an index entry.
func (f *FileIndex) Upsert(entry Entry) error {
	if len(entry.Vector) == 0 {
		return fmt.Errorf("vector: missing embedding for %s", entry.SourceID)
	}
	if f.embeddingDim == 0 {
		f.embeddingDim = len(entry.Vector)
	}
	if len(entry.Vector) != f.embeddingDim {
		return fmt.Errorf("vector: embedding dim mismatch: %d != %d", len(entry.Vector), f.embeddingDim)
	}
	if f.embeddingModel == "" {
		f.embeddingModel = entry.EmbeddingModel
	}

	key := entryKey(entry.SourceType, entry.SourceID)
	ref := entry.VectorRef
	if ref == "" {
		ref = strconv.Itoa(len(f.vectors))
		entry.VectorRef = ref
		f.vectors = append(f.vectors, entry.Vector)
	} else {
		idx, err := strconv.Atoi(ref)
		if err != nil {
			return fmt.Errorf("vector: invalid vector_ref: %w", err)
		}
		if idx >= len(f.vectors) {
			return errors.New("vector: vector_ref out of range")
		}
		f.vectors[idx] = entry.Vector
	}
	f.entries[key] = entry
	f.refToKey[ref] = key
	if f.graph != nil {
		_ = f.graph.Insert(entry.ID, entry.Vector)
	}
	return nil
}

// Delete removes an entry from the in-memory index.
func (f *FileIndex) Delete(sourceType SourceType, sourceID string) error {
	key := entryKey(sourceType, sourceID)
	entry, ok := f.entries[key]
	if !ok {
		return nil
	}
	delete(f.entries, key)
	if entry.VectorRef != "" {
		delete(f.refToKey, entry.VectorRef)
	}
	return nil
}

// Search returns the top-k most similar entries to the query vector.
func (f *FileIndex) Search(query []float32, k int) ([]SearchResult, error) {
	if len(f.entries) == 0 || len(f.vectors) == 0 {
		return nil, nil
	}
	if k <= 0 {
		return nil, nil
	}
	if f.embeddingDim > 0 && len(query) != f.embeddingDim {
		return nil, errors.New("vector: query dim mismatch")
	}

	if f.graph != nil {
		ids, scores, err := f.graph.Search(query, k)
		if err == nil && len(ids) > 0 {
			results := make([]SearchResult, 0, len(ids))
			for i, id := range ids {
				key := f.entryKeyByID(id)
				if key == "" {
					continue
				}
				entry := f.entries[key]
				results = append(results, SearchResult{Entry: entry, Score: scores[i]})
			}
			if len(results) > 0 {
				return results, nil
			}
		}
	}
	return f.linearSearch(query, k), nil
}

// Rebuild discards all entries and rebuilds the index.
func (f *FileIndex) Rebuild(entries []Entry) error {
	f.entries = make(map[string]Entry)
	f.refToKey = make(map[string]string)
	f.vectors = nil
	if f.graph != nil {
		_ = f.graph.Deserialize(nil)
	}
	for _, entry := range entries {
		if err := f.Upsert(entry); err != nil {
			return err
		}
	}
	return f.Flush()
}

// Flush writes the index to disk.
func (f *FileIndex) Flush() error {
	if f.path == "" {
		return nil
	}
	if f.codec == nil {
		return errors.New("vector: codec not configured")
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("vector: ensure index dir: %w", err)
	}
	tmp := f.path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("vector: create index: %w", err)
	}

	flat := flattenVectors(f.vectors)
	header := vecfile.Header{
		ProfileID:      f.profileID,
		EmbeddingModel: f.embeddingModel,
		EmbeddingDim:   uint32(f.embeddingDim),
		EntryCount:     uint32(len(f.vectors)),
	}
	if err := f.codec.WriteHeader(file, header); err != nil {
		file.Close()
		return fmt.Errorf("vector: write header: %w", err)
	}
	if err := f.codec.WriteVectors(file, flat, f.embeddingDim); err != nil {
		file.Close()
		return fmt.Errorf("vector: write vectors: %w", err)
	}
	var graphBytes []byte
	if f.graph != nil {
		graphBytes, err = f.graph.Serialize()
		if err != nil {
			file.Close()
			return fmt.Errorf("vector: serialize graph: %w", err)
		}
	}
	if err := f.codec.WriteGraph(file, graphBytes); err != nil {
		file.Close()
		return fmt.Errorf("vector: write graph: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("vector: close index: %w", err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("vector: move index: %w", err)
	}
	return nil
}

// Close releases resources held by the index.
func (f *FileIndex) Close() error { return nil }

func (f *FileIndex) linearSearch(query []float32, k int) []SearchResult {
	scores := make([]SearchResult, 0, len(f.vectors))
	for idx, vector := range f.vectors {
		score := cosineSimilarity(query, vector)
		ref := strconv.Itoa(idx)
		key, ok := f.refToKey[ref]
		if !ok {
			continue
		}
		entry := f.entries[key]
		scores = append(scores, SearchResult{Entry: entry, Score: score})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].Score > scores[j].Score })
	if len(scores) > k {
		scores = scores[:k]
	}
	return scores
}

// SeedEntries loads manifest entries into the index lookup maps.
func (f *FileIndex) SeedEntries(entries []Entry) {
	for _, entry := range entries {
		key := entryKey(entry.SourceType, entry.SourceID)
		f.entries[key] = entry
		if entry.VectorRef != "" {
			f.refToKey[entry.VectorRef] = key
		}
	}
}

func (f *FileIndex) entryKeyByID(id string) string {
	for key, entry := range f.entries {
		if entry.ID == id {
			return key
		}
	}
	return ""
}

func entryKey(sourceType SourceType, sourceID string) string {
	return string(sourceType) + ":" + sourceID
}

func flattenVectors(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	dim := len(vectors[0])
	out := make([]float32, 0, len(vectors)*dim)
	for _, v := range vectors {
		out = append(out, v...)
	}
	return out
}

func reshapeVectors(flat []float32, count int, dim int) [][]float32 {
	if count == 0 || dim == 0 {
		return nil
	}
	vectors := make([][]float32, 0, count)
	for i := 0; i < count; i++ {
		start := i * dim
		end := start + dim
		if end > len(flat) {
			break
		}
		vec := make([]float32, dim)
		copy(vec, flat[start:end])
		vectors = append(vectors, vec)
	}
	return vectors
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
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
