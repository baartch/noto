package vector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/provider"
	"noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

type atomicRebuildEmbedder struct{}

func (atomicRebuildEmbedder) Embed(context.Context, provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{Embedding: []float32{0, 1, 0}}, nil
}

type atomicRebuildManifest struct {
	model         string
	dim           int
	status        string
	entries       []*ManifestEntry
	beforeReplace func()
}

func (m *atomicRebuildManifest) SetManifestStatusStr(_ context.Context, _ string, status string) error {
	m.status = status
	return nil
}

func (m *atomicRebuildManifest) UpsertEntry(_ context.Context, entry *ManifestEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *atomicRebuildManifest) ListEntriesVec(context.Context, string) ([]*ManifestEntry, error) {
	return m.entries, nil
}

func (m *atomicRebuildManifest) GetEmbeddingState(context.Context, string) (string, int, string, error) {
	return m.model, m.dim, m.status, nil
}

func (m *atomicRebuildManifest) ReplaceEntriesAndEmbedding(_ context.Context, _ string, model string, dim int, entries []*ManifestEntry) error {
	m.model, m.dim, m.entries = model, dim, entries
	if m.beforeReplace != nil {
		m.beforeReplace()
	}
	return nil
}

func TestRebuilderRestoresManifestIfStagedIndexCannotPublish(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "memory.vec")
	index := NewFileIndex(indexPath, file.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	index.WithProfile("p")
	if err := index.Upsert(Entry{
		ID: "ve-old", ProfileID: "p", SourceType: SourceMemoryNote, SourceID: "old",
		ChunkHash: "old-hash", EmbeddingModel: "old-model", Vector: []float32{1, 0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := index.Flush(); err != nil {
		t.Fatal(err)
	}
	manifest := &atomicRebuildManifest{
		model: "old-model", dim: 2, status: string(ManifestReady),
		entries: []*ManifestEntry{{ID: "ve-old", ProfileID: "p", SourceType: SourceMemoryNote, SourceID: "old", ChunkHash: "old-hash", EmbeddingModel: "old-model", EmbeddingDim: 2, VectorRef: "0"}},
	}
	damaged := false
	manifest.beforeReplace = func() {
		if damaged {
			return
		}
		damaged = true
		if err := os.Remove(indexPath); err != nil {
			t.Errorf("remove index to force publish error: %v", err)
			return
		}
		if err := os.Mkdir(indexPath, 0o755); err != nil {
			t.Errorf("create directory to force publish error: %v", err)
		}
	}

	rebuilder := NewRebuilder(manifest, index, "p").
		WithManifest(manifest).
		WithEmbedder(atomicRebuildEmbedder{}, "new-model").
		WithPreviousStatus(string(ManifestReady))
	err := rebuilder.Rebuild(context.Background(), []MemoryNoteRecord{{ID: "new", Content: "new note"}})
	if err == nil {
		t.Fatal("Rebuild succeeded after staged index publication was made impossible")
	}
	if manifest.model != "old-model" || manifest.dim != 2 || manifest.status != string(ManifestReady) || len(manifest.entries) != 1 || manifest.entries[0].SourceID != "old" {
		t.Fatalf("manifest after failed publication = %#v; want original ready state", manifest)
	}
	results, searchErr := index.Search([]float32{1, 0}, 1)
	if searchErr != nil || len(results) != 1 || results[0].Entry.SourceID != "old" {
		t.Fatalf("live index after failed publication = %#v, %v; want old entry", results, searchErr)
	}
}

func TestPreparedRebuildCommitRejectsSecondUse(t *testing.T) {
	index := NewFileIndex("", file.NewBinaryCodec(), nil)
	prepared, err := index.PrepareRebuild(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Commit(); err == nil || err.Error() != "vector: prepared rebuild is not active" {
		t.Fatalf("second Commit() error = %v, want inactive rebuild error", err)
	}
}
