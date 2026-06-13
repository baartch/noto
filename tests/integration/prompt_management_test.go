package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/config"
	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestPromptStore_GetDefault_WhenMissing(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	ps := profile.NewPromptStore("test-profile", nil)

	content, err := ps.GetSystemPrompt(context.Background())
	if err != nil {
		t.Fatalf("GetSystemPrompt: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty default prompt")
	}
}

func TestPromptStore_SetAndGet(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	ps := profile.NewPromptStore("test-profile", nil)

	custom := "You are a specialized assistant for software architecture."
	if err := ps.SetSystemPrompt(context.Background(), custom); err != nil {
		t.Fatalf("SetSystemPrompt: %v", err)
	}

	got, err := ps.GetSystemPrompt(context.Background())
	if err != nil {
		t.Fatalf("GetSystemPrompt after set: %v", err)
	}
	if got != custom {
		t.Errorf("prompt mismatch: got %q, want %q", got, custom)
	}
}

func TestPromptStore_UpdateOverwrites(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	ps := profile.NewPromptStore("test-profile", nil)
	ctx := context.Background()

	if err := ps.SetSystemPrompt(ctx, "version 1"); err != nil {
		t.Fatal(err)
	}
	if err := ps.SetSystemPrompt(ctx, "version 2"); err != nil {
		t.Fatal(err)
	}

	got, err := ps.GetSystemPrompt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "version 2" {
		t.Errorf("expected version 2, got %q", got)
	}
}

func TestPromptStore_AutoBootstrapsMissingPromptFile(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	slug := "bootstrap-profile"
	ps := profile.NewPromptStore(slug, nil)

	if _, err := ps.GetSystemPrompt(context.Background()); err != nil {
		t.Fatalf("initial GetSystemPrompt: %v", err)
	}

	systemPath, err := config.ProfileSystemPromptPath(slug)
	if err != nil {
		t.Fatalf("ProfileSystemPromptPath: %v", err)
	}
	if err := os.Remove(systemPath); err != nil {
		t.Fatalf("remove system prompt: %v", err)
	}

	content, err := ps.GetSystemPrompt(context.Background())
	if err != nil {
		t.Fatalf("GetSystemPrompt after remove: %v", err)
	}
	if content == "" {
		t.Fatal("expected bootstrapped prompt content")
	}
	if _, err := os.Stat(filepath.Clean(systemPath)); err != nil {
		t.Fatalf("expected bootstrapped file to exist: %v", err)
	}
}

func TestPromptStore_PersistsAcrossRestarts_FileBacked(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	slug := "persist-profile"
	ctx := context.Background()

	ps1 := profile.NewPromptStore(slug, nil)
	if err := ps1.SetSystemPrompt(ctx, "persist me"); err != nil {
		t.Fatalf("SetSystemPrompt: %v", err)
	}

	// Simulate restart by constructing a new prompt store instance.
	ps2 := profile.NewPromptStore(slug, nil)
	got, err := ps2.GetSystemPrompt(ctx)
	if err != nil {
		t.Fatalf("GetSystemPrompt after restart: %v", err)
	}
	if got != "persist me" {
		t.Fatalf("expected persisted prompt, got %q", got)
	}
}

func TestPromptStore_LoadsFromFiles_NotDB(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	slug := "file-over-db"
	ps := profile.NewPromptStore(slug, nil)
	if err := ps.SetSystemPrompt(ctx, "file prompt"); err != nil {
		t.Fatalf("SetSystemPrompt: %v", err)
	}

	// Seed conflicting DB prompt content; file-backed store should ignore this.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS system_prompts (
			id TEXT PRIMARY KEY,
			profile_id TEXT NOT NULL UNIQUE,
			prompt TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)
	`); err != nil {
		t.Fatalf("create system_prompts: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO system_prompts (id, profile_id, prompt, created_at, updated_at)
		VALUES ('sp-1', 'irrelevant-profile-id', 'db prompt', datetime('now'), datetime('now'))
		ON CONFLICT(profile_id) DO UPDATE SET prompt='db prompt'
	`); err != nil {
		t.Fatalf("insert db prompt: %v", err)
	}

	got, err := ps.GetSystemPrompt(ctx)
	if err != nil {
		t.Fatalf("GetSystemPrompt: %v", err)
	}
	if got != "file prompt" {
		t.Fatalf("expected file-backed prompt, got %q", got)
	}
}

func TestPromptChange_InvalidatesContextCacheAcrossRestart(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	profSvc := profile.NewService(store.NewProfileRepo(db))
	p, err := profSvc.Create(ctx, "CachePrompt")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	ps := profile.NewPromptStore(p.Slug, nil)
	if err := ps.SetSystemPrompt(ctx, "prompt v1"); err != nil {
		t.Fatalf("set prompt: %v", err)
	}

	noteRepo := store.NewMemoryNoteRepo(db)
	cacheRepo := store.NewContextCacheRepo(db)
	if err := noteRepo.Create(ctx, &store.MemoryNote{ID: "n1", ProfileID: p.ID, Category: store.CategoryFact, Content: "A", Importance: 5, SourceMessageIDs: "[]"}); err != nil {
		t.Fatalf("create note: %v", err)
	}

	r1 := memory.NewRetrieval(noteRepo, cacheRepo)
	ctx1, err := r1.Assemble(ctx, p.ID, "prompt v1")
	if err != nil {
		t.Fatalf("assemble v1: %v", err)
	}
	if ctx1.CacheHit {
		t.Fatal("first assemble should not be cache hit")
	}

	// Simulate restart with a fresh retrieval service and changed prompt.
	if err := ps.SetSystemPrompt(ctx, "prompt v2"); err != nil {
		t.Fatalf("set prompt v2: %v", err)
	}
	r2 := memory.NewRetrieval(noteRepo, cacheRepo)
	ctx2, err := r2.Assemble(ctx, p.ID, "prompt v2")
	if err != nil {
		t.Fatalf("assemble v2: %v", err)
	}
	if ctx2.CacheHit {
		t.Fatal("expected cache miss after prompt change")
	}
}
