package chat

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/startup"
	"noto/internal/store"
	"noto/internal/vector"
	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

type recoveryAdapter struct {
	chatRecordingAdapter
	fail bool
}

type modelChangeAdapter struct {
	chatRecordingAdapter
	failModel         string
	invalidDimensions bool
	calls             []provider.EmbeddingRequest
	onEmbed           func(provider.EmbeddingRequest)
}

func (a *modelChangeAdapter) Embed(ctx context.Context, req provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	a.calls = append(a.calls, req)
	if a.onEmbed != nil {
		a.onEmbed(req)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Model == a.failModel {
		return nil, errors.New("embedding unavailable")
	}
	if req.Model == "new-model" {
		if a.invalidDimensions && req.Input == "second note" {
			return &provider.EmbeddingResponse{Embedding: []float32{0, 1}}, nil
		}
		return &provider.EmbeddingResponse{Embedding: []float32{0, 1, 0}}, nil
	}
	return &provider.EmbeddingResponse{Embedding: []float32{1, 0}}, nil
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
	var events []startup.Event
	s, err := NewSession(ctx, "p", "p", "system", seed.db, seed.convRepo, seed.msgRepo, repo, adapter, nil, observe.NewJSONLogger(io.Discard), nil, nil, nil, nil, func(e startup.Event) { events = append(events, e) })
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
	var sawRebuild, sawCheck bool
	for _, event := range events {
		if event == (startup.Event{Phase: "rebuild", State: "success", Done: 1, Total: 1}) {
			sawRebuild = true
		}
		if event.Phase == "checking" && event.State == "start" {
			sawCheck = true
		}
	}
	if !sawRebuild || !sawCheck {
		t.Fatalf("startup progress missing rebuild: %#v", events)
	}
	before := len(events)
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatal(err)
	}
	if len(events) != before {
		t.Fatalf("later sync emitted startup progress: %#v", events[before:])
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
	var events []startup.Event
	s, err := NewSession(ctx, "p", "p", "system", seed.db, seed.convRepo, seed.msgRepo, repo, adapter, nil, observe.NewJSONLogger(io.Discard), nil, nil, nil, nil, func(e startup.Event) { events = append(events, e) })
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
	var sawRebuild bool
	for _, event := range events {
		if event == (startup.Event{Phase: "rebuild", State: "success", Done: 2, Total: 2}) {
			sawRebuild = true
		}
	}
	if !sawRebuild {
		t.Fatalf("missing startup rebuild for incompatible index model: %#v", events)
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

func TestChangeEmbeddingModelRebuildsAllNotesBeforeSwitching(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	ctx := context.Background()
	notes := []*store.MemoryNote{
		{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "first note", Importance: 5, SourceMessageIDs: "[]"},
		{ID: "two", ProfileID: "p", Category: store.CategoryFact, Content: "second note", Importance: 5, SourceMessageIDs: "[]"},
	}
	for _, note := range notes {
		if err := repo.Create(ctx, note); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.syncVectorIndex(ctx, notes); err != nil {
		t.Fatal(err)
	}
	adapter.calls = nil

	if err := s.ChangeEmbeddingModel(ctx, "new-model"); err != nil {
		t.Fatalf("ChangeEmbeddingModel: %v", err)
	}
	if s.EmbeddingModel() != "new-model" {
		t.Fatalf("EmbeddingModel() = %q, want new-model", s.EmbeddingModel())
	}
	if len(adapter.calls) != len(notes) {
		t.Fatalf("embedding calls = %d, want all %d notes", len(adapter.calls), len(notes))
	}
	for _, call := range adapter.calls {
		if call.Model != "new-model" {
			t.Errorf("embedded with model %q, want new-model", call.Model)
		}
	}
	idx := vector.NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
	if err := idx.Load(); err != nil {
		t.Fatal(err)
	}
	for _, note := range notes {
		ref, ok := idx.VectorRefFor(vector.SourceMemoryNote, note.ID)
		if !ok {
			t.Errorf("rebuilt index missing note %s", note.ID)
		}
		_ = ref
	}
	results, err := idx.Search([]float32{0, 1, 0}, len(notes))
	if err != nil || len(results) != len(notes) {
		t.Fatalf("search rebuilt index = %d results, %v; want %d", len(results), err, len(notes))
	}
	if _, err := idx.Search([]float32{1, 0}, len(notes)); err == nil {
		t.Fatal("index accepted old-model query dimensions after switch")
	}
	entries, err := store.NewVectorManifestRepo(s.db).ListEntriesVec(ctx, "p")
	if err != nil || len(entries) != len(notes) {
		t.Fatalf("manifest entries = %d, %v; want %d", len(entries), err, len(notes))
	}
	for _, entry := range entries {
		if entry.EmbeddingModel != "new-model" || entry.EmbeddingDim != 3 {
			t.Errorf("manifest entry = model %q dim %d, want new-model dim 3", entry.EmbeddingModel, entry.EmbeddingDim)
		}
	}
}

func TestChangeEmbeddingModelFailurePreservesOldIndexAndModel(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	ctx := context.Background()
	note := &store.MemoryNote{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "original note", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	adapter.failModel = "new-model"
	if err := s.ChangeEmbeddingModel(ctx, "new-model"); err == nil {
		t.Fatal("ChangeEmbeddingModel succeeded, want embedding error")
	}
	if got := s.EmbeddingModel(); got != "old-model" {
		t.Fatalf("EmbeddingModel() = %q after failed rebuild, want old-model", got)
	}
	after, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("failed rebuild changed index bytes: err=%v equal=%t", err, reflect.DeepEqual(after, before))
	}
}

func TestChangeEmbeddingModelDimensionMismatchPreservesOldIndex(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	ctx := context.Background()
	notes := []*store.MemoryNote{
		{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "first note", Importance: 5, SourceMessageIDs: "[]"},
		{ID: "two", ProfileID: "p", Category: store.CategoryFact, Content: "second note", Importance: 5, SourceMessageIDs: "[]"},
	}
	for _, note := range notes {
		if err := repo.Create(ctx, note); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.syncVectorIndex(ctx, notes); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	adapter.invalidDimensions = true
	if err := s.ChangeEmbeddingModel(ctx, "new-model"); err == nil {
		t.Fatal("ChangeEmbeddingModel succeeded with inconsistent dimensions")
	}
	if got := s.EmbeddingModel(); got != "old-model" {
		t.Fatalf("EmbeddingModel() = %q after dimension failure, want old-model", got)
	}
	after, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("dimension failure changed index bytes: err=%v equal=%t", err, reflect.DeepEqual(after, before))
	}
}

func TestChangeEmbeddingModelNoopAndEmptyNotes(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, _ := newSaveNoteTestSession(t, adapter, false, nil)
	s.embeddingModel = "old-model"
	if err := s.ChangeEmbeddingModel(context.Background(), "old-model"); err != nil {
		t.Fatalf("unchanged model: %v", err)
	}
	if len(adapter.calls) != 0 {
		t.Fatalf("unchanged model made %d embedding calls", len(adapter.calls))
	}
	s.vectorIndexPath = filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndex = vector.NewFileIndex(s.vectorIndexPath, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	if err := s.ChangeEmbeddingModel(context.Background(), "new-model"); err != nil {
		t.Fatalf("empty note rebuild: %v", err)
	}
	if len(adapter.calls) != 0 {
		t.Fatalf("empty note rebuild made %d embedding calls", len(adapter.calls))
	}
	idx := vector.NewFileIndex(s.vectorIndexPath, vecfile.NewBinaryCodec(), nil)
	if err := idx.Load(); err != nil {
		t.Fatal(err)
	}
	if got := idx.EmbeddingModel(); got != "new-model" {
		t.Fatalf("empty index model = %q, want new-model", got)
	}
}

func TestChangeEmbeddingModelClearsVectorIndexWhenDisabled(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	ctx := context.Background()
	note := &store.MemoryNote{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "original note", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatal(err)
	}
	adapter.calls = nil
	if err := s.ChangeEmbeddingModel(ctx, ""); err != nil {
		t.Fatalf("disable embeddings: %v", err)
	}
	if s.EmbeddingModel() != "" || !s.EmbeddingModelMissingActive() {
		t.Fatalf("embedding state = %q missing=%t; want disabled", s.EmbeddingModel(), s.EmbeddingModelMissingActive())
	}
	if len(adapter.calls) != 0 {
		t.Fatalf("disabling embeddings made %d provider calls", len(adapter.calls))
	}
	idx := vector.NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
	if err := idx.Load(); err != nil {
		t.Fatal(err)
	}
	if idx.EmbeddingModel() != "" {
		t.Fatalf("index model = %q; want empty", idx.EmbeddingModel())
	}
	manifest, err := store.NewVectorManifestRepo(s.db).GetManifest(ctx, "p")
	if err != nil || manifest.EmbeddingModel != "" || manifest.EmbeddingDim != 0 {
		t.Fatalf("manifest = %#v, %v; want empty model and dimension", manifest, err)
	}
	entries, err := store.NewVectorManifestRepo(s.db).ListEntriesVec(ctx, "p")
	if err != nil || len(entries) != 0 {
		t.Fatalf("manifest entries = %#v, %v; want none", entries, err)
	}
}

func TestChangeEmbeddingModelCancellationPreservesOldModel(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	s.vectorIndexPath = filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndex = vector.NewFileIndex(s.vectorIndexPath, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	note := &store.MemoryNote{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "note", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(context.Background(), note); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.ChangeEmbeddingModel(ctx, "new-model"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ChangeEmbeddingModel error = %v, want context.Canceled", err)
	}
	if got := s.EmbeddingModel(); got != "old-model" {
		t.Fatalf("EmbeddingModel() = %q after cancellation, want old-model", got)
	}
}

func TestChangeEmbeddingModelCancellationDoesNotAlterManifest(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	s.vectorIndexPath = filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndex = vector.NewFileIndex(s.vectorIndexPath, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	note := &store.MemoryNote{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "note", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(context.Background(), note); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(context.Background(), []*store.MemoryNote{note}); err != nil {
		t.Fatal(err)
	}
	manifestRepo := store.NewVectorManifestRepo(s.db)
	before, err := manifestRepo.GetManifest(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	adapter.onEmbed = func(provider.EmbeddingRequest) { cancel() }
	err = s.ChangeEmbeddingModel(ctx, "new-model")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ChangeEmbeddingModel error = %v, want context.Canceled", err)
	}
	after, err := manifestRepo.GetManifest(context.Background(), "p")
	if err != nil || after.Status != before.Status || after.EmbeddingModel != before.EmbeddingModel {
		t.Fatalf("manifest after cancellation = %#v, %v; before = %#v", after, err, before)
	}
}

func TestChangeEmbeddingModelRejectsNotesChangedDuringRebuild(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	ctx := context.Background()
	note := &store.MemoryNote{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "original note", Importance: 5, SourceMessageIDs: "[]"}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatal(err)
	}
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{note}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	adapter.onEmbed = func(provider.EmbeddingRequest) {
		note.Content = "updated while rebuilding"
		if err := repo.Update(ctx, note); err != nil {
			t.Errorf("update note during embedding: %v", err)
		}
	}
	if err := s.ChangeEmbeddingModel(ctx, "new-model"); err == nil {
		t.Fatal("ChangeEmbeddingModel succeeded after note source changed")
	}
	if got := s.EmbeddingModel(); got != "old-model" {
		t.Fatalf("EmbeddingModel() = %q after source change, want old-model", got)
	}
	after, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("source change modified index bytes: err=%v equal=%t", err, reflect.DeepEqual(after, before))
	}
}

func TestSyncVectorIndexRebuildsAllNotesWhenEmbeddingModelChanges(t *testing.T) {
	adapter := &modelChangeAdapter{}
	s, repo := newSaveNoteTestSession(t, adapter, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	s.embeddingModel = "old-model"
	ctx := context.Background()
	notes := []*store.MemoryNote{
		{ID: "one", ProfileID: "p", Category: store.CategoryFact, Content: "first note", Importance: 5, SourceMessageIDs: "[]"},
		{ID: "two", ProfileID: "p", Category: store.CategoryFact, Content: "second note", Importance: 5, SourceMessageIDs: "[]"},
	}
	for _, note := range notes {
		if err := repo.Create(ctx, note); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.syncVectorIndex(ctx, notes); err != nil {
		t.Fatal(err)
	}
	adapter.calls = nil
	s.embeddingModel = "new-model"
	if err := s.syncVectorIndex(ctx, notes); err != nil {
		t.Fatalf("sync after model change: %v", err)
	}
	if len(adapter.calls) != len(notes) {
		t.Fatalf("re-embedding calls = %d, want %d", len(adapter.calls), len(notes))
	}
	for _, call := range adapter.calls {
		if call.Model != "new-model" {
			t.Errorf("re-embedded with model %q, want new-model", call.Model)
		}
	}
}

func TestStartupSyncReportsEarlyFailure(t *testing.T) {
	s, _ := newSaveNoteTestSession(t, &recoveryAdapter{}, false, nil)
	path := filepath.Join(t.TempDir(), "memory.vec")
	s.vectorIndexPath = path
	s.vectorIndex = vector.NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
	s.embeddingModel = "embed"
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	var events []startup.Event
	err := s.syncVectorIndex(context.Background(), nil, func(e startup.Event) { events = append(events, e) })
	if err == nil || len(events) == 0 || events[len(events)-1].State != "failure" {
		t.Fatalf("sync = %v, events = %#v; want failure progress", err, events)
	}
}
