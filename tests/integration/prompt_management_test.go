package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/config"
	"noto/internal/profile"
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
