package integration

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"noto/internal/commands"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/internal/suggest"
	"noto/internal/vector"
	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

// BenchmarkStartup measures profile list retrieval (startup critical path).
func BenchmarkStartup_ProfileList(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("NOTO_APP_DIR", dir)
	db, err := store.OpenForTesting(filepath.Join(dir, "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	svc := profile.NewService(store.NewProfileRepo(db))
	ctx := context.Background()
	for i := range 10 {
		svc.Create(ctx, "Profile "+string(rune('A'+i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.List(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSlashSuggestion measures suggestion refresh latency per keystroke.
func BenchmarkSlashSuggestion_Refresh(b *testing.B) {
	r := commands.NewRegistry()
	commands.RegisterPromptCommands(r)
	engine := suggest.New(r)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Suggest("prompt")
	}
}

// BenchmarkContextCache_GetMiss measures cache miss lookup speed.
func BenchmarkContextCache_GetMiss(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("NOTO_APP_DIR", dir)
	db, err := store.OpenForTesting(filepath.Join(dir, "bench_cache.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	cacheRepo := store.NewContextCacheRepo(db)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cacheRepo.Get(ctx, "nonexistent-profile", "nonexistent-key")
	}
}

func BenchmarkVectorLookup_P95(b *testing.B) {
	index := vector.NewFileIndex("", vecfile.NewNoopCodec(), hnsw.NewNoopGraph())
	entryCount := 5000
	dim := 8

	entries := make([]vector.Entry, 0, entryCount)
	for i := range entryCount {
		vec := make([]float32, dim)
		for j := range dim {
			vec[j] = float32(i+j) / 1000
		}
		entries = append(entries, vector.Entry{
			ID:             "ve-" + strconv.Itoa(i),
			ProfileID:      "bench",
			SourceType:     vector.SourceMemoryNote,
			SourceID:       "note-" + strconv.Itoa(i),
			ChunkHash:      "hash",
			EmbeddingModel: "bench",
			EmbeddingDim:   dim,
			Vector:         vec,
		})
	}

	if err := index.Rebuild(entries); err != nil {
		b.Fatalf("rebuild: %v", err)
	}

	query := make([]float32, dim)
	for j := range dim {
		query[j] = 0.1
	}

	b.ResetTimer()
	var samples []time.Duration
	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, _ = index.Search(query, 10)
		samples = append(samples, time.Since(start))
	}

	if len(samples) == 0 {
		return
	}
	p95 := percentile(samples, 0.95)
	if p95 > 40*time.Millisecond {
		b.Fatalf("p95 vector lookup too slow: %s", p95)
	}
}

func percentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	idx := int(float64(len(samples)-1) * p)
	idx = max(idx, 0)
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	slices.Sort(samples)
	return samples[idx]
}
