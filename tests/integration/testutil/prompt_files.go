package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// EnsurePromptFiles creates profile-local prompt files and returns their paths.
func EnsurePromptFiles(t *testing.T, profileDir, systemPrompt, extractorPrompt string) (string, string) {
	t.Helper()

	promptsDir := filepath.Join(profileDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("create prompts dir: %v", err)
	}

	systemPath := filepath.Join(promptsDir, "system.md")
	extractorPath := filepath.Join(promptsDir, "extractor.md")

	if err := os.WriteFile(systemPath, []byte(systemPrompt), 0o644); err != nil {
		t.Fatalf("write system prompt: %v", err)
	}
	if err := os.WriteFile(extractorPath, []byte(extractorPrompt), 0o644); err != nil {
		t.Fatalf("write extractor prompt: %v", err)
	}

	return systemPath, extractorPath
}

// ReadPromptFile reads a prompt file for assertions in integration tests.
func ReadPromptFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prompt file %s: %v", path, err)
	}
	return string(b)
}
