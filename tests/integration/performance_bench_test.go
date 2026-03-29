package integration

import (
	"context"
	"path/filepath"
	"testing"

	"noto/internal/commands"
	"noto/internal/parser"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/internal/suggest"
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
	commands.RegisterProfileCommands(r, commands.NoopProfileService{})
	commands.RegisterPromptCommands(r)
	engine := suggest.New(r)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Suggest("profile")
	}
}

// BenchmarkSlashParser measures parsing latency per input.
func BenchmarkSlashParser_Parse(b *testing.B) {
	inputs := []string{
		"/profile list",
		"/profile select \"My Profile\"",
		"/prompt show",
		"/pro",
		"/",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := inputs[i%len(inputs)]
		_ = parser.Parse(input)
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
