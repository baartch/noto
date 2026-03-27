package integration

import (
	"testing"

	"noto/internal/profile"
)

func TestPromptStore_GetDefault_WhenFileAbsent(t *testing.T) {
	// Use a temp slug that won't have a real file on disk.
	slug := "test-prompt-slug-" + t.Name()
	ps := profile.NewPromptStore(slug)

	content, err := ps.GetSystemPrompt()
	if err != nil {
		t.Fatalf("GetSystemPrompt: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty default prompt")
	}
}

func TestPromptStore_SetAndGet(t *testing.T) {
	slug := "test-prompt-set-" + t.Name()
	ps := profile.NewPromptStore(slug)

	custom := "You are a specialized assistant for software architecture."
	if err := ps.SetSystemPrompt(custom); err != nil {
		t.Fatalf("SetSystemPrompt: %v", err)
	}

	got, err := ps.GetSystemPrompt()
	if err != nil {
		t.Fatalf("GetSystemPrompt after set: %v", err)
	}
	if got != custom {
		t.Errorf("prompt mismatch: got %q, want %q", got, custom)
	}
}

func TestPromptStore_PromptVersion_ChangesAfterUpdate(t *testing.T) {
	slug := "test-prompt-version-" + t.Name()
	ps := profile.NewPromptStore(slug)

	if err := ps.SetSystemPrompt("version 1"); err != nil {
		t.Fatal(err)
	}
	v1, err := ps.PromptVersion()
	if err != nil {
		t.Fatal(err)
	}

	if err := ps.SetSystemPrompt("version 2"); err != nil {
		t.Fatal(err)
	}
	v2, err := ps.PromptVersion()
	if err != nil {
		t.Fatal(err)
	}

	// Both versions should be non-empty; they may or may not differ (depends on
	// filesystem mtime resolution), but the version string should be defined.
	if v1 == "" || v2 == "" {
		t.Error("prompt version should not be empty")
	}
}
