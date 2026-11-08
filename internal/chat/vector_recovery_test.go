package chat

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/vector"
	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

type recoveryAdapter struct {
	chatRecordingAdapter
	fail bool
}

func (a *recoveryAdapter) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	if a.fail {
		return nil, errors.New("embedding unavailable")
	}
	return &provider.EmbeddingResponse{Embedding: []float32{1, 0}}, nil
}

func TestSyncVectorIndexRebuildsAllNotesWhenIndexUnavailable(t *testing.T) {
	for _, corrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "corrupt"}[corrupted], func(t *testing.T) {
			adapter := &recoveryAdapter{}
			s, repo := newSaveNoteTestSession(t, adapter, false, nil)
			path := filepath.Join(t.TempDir(), "memory.vec")
			s.vectorIndexPath = path
			s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
			s.embeddingModel = "embedding-model"
			ctx := context.Background()
			notes := []*store.MemoryNote{
				{ID: "old", ProfileID: "p", Category: store.CategoryFact, Content: "old note", Importance: 5, SourceMessageIDs: "[]"},
				{ID: "new", ProfileID: "p", Category: store.CategoryFact, Content: "new note", Importance: 5, SourceMessageIDs: "[]"},
			}
			for _, note := range notes {
				if err := repo.Create(ctx, note); err != nil {
					t.Fatal(err)
				}
			}
			if corrupted {
				if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.syncVectorIndex(ctx, notes[1:]); err != nil {
				t.Fatal(err)
			}
			idx := vector.NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
			if err := idx.Load(); err != nil {
				t.Fatal(err)
			}
			for _, note := range notes {
				if _, ok := idx.VectorRefFor(vector.SourceMemoryNote, note.ID); !ok {
					t.Errorf("recovered index missing %s", note.ID)
				}
			}
			manifest, err := store.NewVectorManifestRepo(s.db).ListEntriesVec(ctx, "p")
			if err != nil || len(manifest) != len(notes) {
				t.Fatalf("recovered manifest has %d entries, %v; want %d", len(manifest), err, len(notes))
			}
		})
	}
}

func TestSyncVectorIndexPreservesCorruptFileIfEmbeddingFails(t *testing.T) {
	adapter := &recoveryAdapter{fail: true}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "embedding-model"
	note := &store.MemoryNote{ID: "a", ProfileID: "p", Category: store.CategoryFact, Content: "note", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(context.Background(), note); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(context.Background(), []*store.MemoryNote{note}); err == nil {
		t.Fatal("expected embedding failure")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "broken" {
		t.Fatalf("original file = %q, %v", data, err)
	}
}

func TestSyncVectorIndexReusesUnchangedEmbedding(t *testing.T) {
	adapter := &recoveryAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "embedding-model"
	note := &store.MemoryNote{ID: "a", ProfileID: "p", Category: store.CategoryFact, Content: "note", Importance: 5, SourceMessageIDs: "[]"}
	ctx := context.Background()
	if err := repo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatal(err)
	}
	adapter.fail = true
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatalf("sync unchanged note: %v", err)
	}
}

func TestNewSessionRepairsCorruptIndex(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	adapter := &recoveryAdapter{}
	seed, repo := newSaveNoteTestSession(t, adapter, false, nil)
	ctx := context.Background()
	note := &store.MemoryNote{ID: "a", ProfileID: "p", Category: store.CategoryFact, Content: "note", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}
	if err := store.NewProviderConfigRepo(seed.db).Upsert(ctx, &store.ProviderConfig{ID: "provider", ProfileID: "p", ProviderType: "test", EmbeddingsModel: "embed", IsActive: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.Getenv("NOTO_APP_DIR"), "profiles", "p", "memory.vec")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(ctx, "p", "p", "system", seed.db, seed.convRepo, seed.msgRepo, repo, adapter, nil, observe.NewJSONLogger(io.Discard), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx)
	idx := vector.NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
	if err := idx.Load(); err != nil {
		t.Fatalf("startup did not repair index: %v", err)
	}
	if _, ok := idx.VectorRefFor(vector.SourceMemoryNote, note.ID); !ok {
		t.Fatal("startup recovery omitted note")
	}
}

func TestNewSessionRepairsIncompleteIndex(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	adapter := &recoveryAdapter{}
	seed, repo := newSaveNoteTestSession(t, adapter, false, nil)
	ctx := context.Background()
	for _, id := range []string{"old", "new"} {
		note := &store.MemoryNote{ID: id, ProfileID: "p", Category: store.CategoryFact, Content: id, Importance: 5, SourceMessageIDs: "[]"}
		if err := repo.Create(ctx, note); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.NewProviderConfigRepo(seed.db).Upsert(ctx, &store.ProviderConfig{ID: "provider", ProfileID: "p", ProviderType: "test", EmbeddingsModel: "embed", IsActive: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.Getenv("NOTO_APP_DIR"), "profiles", "p", "memory.vec")
	idx := vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	if err := idx.Upsert(vector.Entry{ID: "ve-new", SourceType: vector.SourceMemoryNote, SourceID: "new", Vector: []float32{1, 0}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.Flush(); err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(ctx, "p", "p", "system", seed.db, seed.convRepo, seed.msgRepo, repo, adapter, nil, observe.NewJSONLogger(io.Discard), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(ctx)
	idx = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
	if err := idx.Load(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"old", "new"} {
		if _, ok := idx.VectorRefFor(vector.SourceMemoryNote, id); !ok {
			t.Errorf("startup recovery omitted %s", id)
		}
	}
}

func TestSyncVectorIndexRebuildUpdatesStaleManifest(t *testing.T) {
	adapter := &recoveryAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "embed"
	ctx := context.Background()
	note := &store.MemoryNote{ID: "a", ProfileID: "p", Category: store.CategoryFact, Content: "old", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatal(err)
	}
	note.Content = "new"
	if err := repo.Update(ctx, note); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatalf("rebuild with stale manifest: %v", err)
	}
	entries, err := store.NewVectorManifestRepo(s.db).ListEntriesVec(ctx, "p")
	if err != nil || len(entries) != 1 || entries[0].ChunkHash != vector.ContentHash("new") {
		t.Fatalf("manifest after rebuild = %#v, %v; want updated hash", entries, err)
	}
}
