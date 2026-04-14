package integration

import (
	"context"
	"testing"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/vector"

	"noto/tests/integration/testutil"
)

type stubVectorIndex struct {
	results []vector.SearchResult
}

func (s *stubVectorIndex) Upsert(_ vector.Entry) error                { return nil }
func (s *stubVectorIndex) Delete(_ vector.SourceType, _ string) error { return nil }
func (s *stubVectorIndex) Search(_ []float32, _ int) ([]vector.SearchResult, error) {
	return s.results, nil
}
func (s *stubVectorIndex) Rebuild(_ []vector.Entry) error { return nil }
func (s *stubVectorIndex) Flush() error                   { return nil }
func (s *stubVectorIndex) Close() error                   { return nil }

type stubRetrievalEmbedder struct{}

func (s *stubRetrievalEmbedder) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{Embedding: []float32{0.1, 0.2}, Model: "stub"}, nil
}

func TestRetrievalPipeline_RanksNotes(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewSessionSummaryRepo(db)
	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Retrieval")

	_ = noteRepo.Create(ctx, &store.MemoryNote{ID: "n1", ProfileID: p.ID, Category: store.CategoryFact, Content: "Alpha", Importance: 5, SourceMessageIDs: "[]"})
	_ = noteRepo.Create(ctx, &store.MemoryNote{ID: "n2", ProfileID: p.ID, Category: store.CategoryFact, Content: "Beta", Importance: 7, SourceMessageIDs: "[]"})

	index := &stubVectorIndex{results: []vector.SearchResult{{Entry: vector.Entry{SourceType: vector.SourceMemoryNote, SourceID: "n2"}, Score: 0.9}}}
	index.results[0].Entry.ProfileID = p.ID
	retrieval := memory.NewRetrieval(
		noteRepo,
		summaryRepo,
		nil,
		memory.WithVectorRetrieval(index, p.ID, &stubRetrievalEmbedder{}, "embed"),
	)

	ctxOut, err := retrieval.Assemble(ctx, p.ID, "system")
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if ctxOut.MemoryBlock == "" {
		t.Fatalf("expected memory block")
	}
	if ctxOut.AssembledPrompt == ctxOut.SystemPrompt {
		t.Fatalf("expected assembled prompt to include memory")
	}
}
