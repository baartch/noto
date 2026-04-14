package integration

import (
	"context"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestPromptStore_GetDefault_WhenMissing(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()

	repo := store.NewSystemPromptRepo(db)
	ps := profile.NewPromptStore("test-profile-id", repo)

	content, err := ps.GetSystemPrompt(context.Background())
	if err != nil {
		t.Fatalf("GetSystemPrompt: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty default prompt")
	}
}

func TestPromptStore_SetAndGet(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()

	repo := store.NewSystemPromptRepo(db)
	ps := profile.NewPromptStore("test-profile-id", repo)

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
	db, closeDB := testutil.TempDB(t)
	defer closeDB()

	repo := store.NewSystemPromptRepo(db)
	ps := profile.NewPromptStore("test-profile-id", repo)
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
