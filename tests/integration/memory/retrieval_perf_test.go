package integration

import (
	"context"
	"slices"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/vector"
	"noto/tests/integration/testutil"
)

type perfVectorIndex struct {
	results []vector.SearchResult
}

func (s *perfVectorIndex) Upsert(_ vector.Entry) error                { return nil }
func (s *perfVectorIndex) Delete(_ vector.SourceType, _ string) error { return nil }
func (s *perfVectorIndex) Search(_ []float32, _ int) ([]vector.SearchResult, error) {
	return s.results, nil
}
func (s *perfVectorIndex) Rebuild(_ []vector.Entry) error { return nil }
func (s *perfVectorIndex) Flush() error                   { return nil }
func (s *perfVectorIndex) Close() error                   { return nil }

type perfEmbedder struct{}

func (s *perfEmbedder) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{Embedding: []float32{0.1, 0.2}, Model: "stub"}, nil
}

func TestRetrievalPipeline_Performance(t *testing.T) {
	const samples = 20
	const budgetMs = int64(2000)

	db, closeDB := testutil.TempDB(t)
	t.Cleanup(closeDB)
	ctx := context.Background()

	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewSessionSummaryRepo(db)
	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "Perf")

	_ = noteRepo.Create(ctx, &store.MemoryNote{ID: "n1", ProfileID: p.ID, Category: store.CategoryFact, Content: "Alpha", Importance: 5, SourceMessageIDs: "[]"})
	_ = noteRepo.Create(ctx, &store.MemoryNote{ID: "n2", ProfileID: p.ID, Category: store.CategoryFact, Content: "Beta", Importance: 7, SourceMessageIDs: "[]"})

	index := &perfVectorIndex{results: []vector.SearchResult{{Entry: vector.Entry{SourceType: vector.SourceMemoryNote, SourceID: "n2", ProfileID: p.ID}, Score: 0.9}}}
	retrieval := memory.NewRetrieval(
		noteRepo,
		summaryRepo,
		nil,
		memory.WithVectorRetrieval(index, p.ID, &perfEmbedder{}, "embed"),
	)

	latencies := make([]int64, 0, samples)
	for range samples {
		start := time.Now()
		_, err := retrieval.Assemble(ctx, p.ID, "system")
		if err != nil {
			t.Fatalf("assemble: %v", err)
		}
		latencies = append(latencies, time.Since(start).Milliseconds())
	}
	slices.Sort(latencies)
	p95 := latencies[int(float64(len(latencies)-1)*0.95)]
	if p95 > budgetMs {
		t.Fatalf("p95 latency %dms exceeded budget %dms", p95, budgetMs)
	}
}
