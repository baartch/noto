package integration

import (
	"context"
	"testing"

	"noto/internal/provider"
	"noto/internal/vector"
)

func TestVectorSync_UpsertAndFlush(t *testing.T) {
	index := &stubIndexSync{}
	profileID := "test-profile"
	syncer := vector.NewSyncer(index, profileID, &stubEmbedder{}, "test-embed")

	ctx := context.Background()
	notes := []vector.MemoryNoteRecord{
		{ID: "n1", Content: "First note content"},
		{ID: "n2", Content: "Second note content"},
	}

	if err := syncer.SyncNotes(ctx, notes); err != nil {
		t.Fatalf("SyncNotes: %v", err)
	}
}

func TestVectorRetrieval_FallsBackToNoteList(t *testing.T) {
	ctx := context.Background()

	notes := []vector.MemoryNoteRecord{
		{ID: "n1", Content: "Alpha"},
		{ID: "n2", Content: "Beta"},
		{ID: "n3", Content: "Gamma"},
	}

	// Use a noteLister stub.
	lister := &stubNoteLister{notes: notes}
	index := &stubIndexSync{}
	retrieval := vector.NewHybridRetrieval(index, lister, "profile-1")

	results, err := retrieval.Retrieve(ctx, []float32{0.1, 0.2}, 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	// NoopIndex returns empty results → fallback to top-2 from list.
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestVectorRetrieval_ContentHash_IsDeterministic(t *testing.T) {
	h1 := vector.ContentHash("hello world")
	h2 := vector.ContentHash("hello world")
	if h1 != h2 {
		t.Error("ContentHash is not deterministic")
	}
	h3 := vector.ContentHash("different")
	if h1 == h3 {
		t.Error("ContentHash collision for different inputs")
	}
}

// ---- stubs ------------------------------------------------------------------

type stubNoteLister struct {
	notes []vector.MemoryNoteRecord
}

func (s *stubNoteLister) ListByProfile(_ context.Context, _ string) ([]vector.MemoryNoteRecord, error) {
	return s.notes, nil
}

type stubIndexSync struct{}

func (s *stubIndexSync) Upsert(_ vector.Entry) error { return nil }
func (s *stubIndexSync) Delete(_ vector.SourceType, _ string) error { return nil }
func (s *stubIndexSync) Search(_ []float32, _ int) ([]vector.SearchResult, error) { return nil, nil }
func (s *stubIndexSync) Rebuild(_ []vector.Entry) error { return nil }
func (s *stubIndexSync) Flush() error { return nil }
func (s *stubIndexSync) Close() error { return nil }

type stubEmbedder struct{}

func (s *stubEmbedder) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{Embedding: []float32{0.1, 0.2}, Model: "stub"}, nil
}
