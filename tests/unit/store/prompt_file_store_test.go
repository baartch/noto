package store_test

import (
	"os"
	"testing"

	"noto/internal/config"
)

func TestPromptFiles_BootstrapAndPersist(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	slug := "prompt-files"

	res, err := config.EnsureProfilePromptFiles(slug)
	if err != nil {
		t.Fatalf("EnsureProfilePromptFiles: %v", err)
	}
	if !res.SystemCreated || !res.ExtractorCreated {
		t.Fatalf("expected both prompt files bootstrapped, got %#v", res)
	}

	systemPath, _ := config.ProfileSystemPromptPath(slug)
	extractorPath, _ := config.ProfileExtractorPromptPath(slug)
	if _, err := os.Stat(systemPath); err != nil {
		t.Fatalf("system prompt missing: %v", err)
	}
	if _, err := os.Stat(extractorPath); err != nil {
		t.Fatalf("extractor prompt missing: %v", err)
	}

	if err := config.WriteSystemPromptFile(slug, "system custom"); err != nil {
		t.Fatalf("WriteSystemPromptFile: %v", err)
	}
	if err := config.WriteExtractorPromptFile(slug, "extractor custom"); err != nil {
		t.Fatalf("WriteExtractorPromptFile: %v", err)
	}

	sys, _, err := config.ReadSystemPromptFile(slug)
	if err != nil {
		t.Fatalf("ReadSystemPromptFile: %v", err)
	}
	ext, _, err := config.ReadExtractorPromptFile(slug)
	if err != nil {
		t.Fatalf("ReadExtractorPromptFile: %v", err)
	}
	if sys != "system custom" {
		t.Fatalf("system mismatch: %q", sys)
	}
	if ext != "extractor custom" {
		t.Fatalf("extractor mismatch: %q", ext)
	}
}

func TestPromptFiles_MissingBootstrapWarningSignal(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	slug := "bootstrap-warning"

	_, err := config.EnsureProfilePromptFiles(slug)
	if err != nil {
		t.Fatalf("initial bootstrap: %v", err)
	}

	systemPath, _ := config.ProfileSystemPromptPath(slug)
	if err := os.Remove(systemPath); err != nil {
		t.Fatalf("remove system prompt: %v", err)
	}

	_, res, err := config.ReadSystemPromptFile(slug)
	if err != nil {
		t.Fatalf("ReadSystemPromptFile: %v", err)
	}
	if !res.SystemCreated {
		t.Fatalf("expected SystemCreated warning signal, got %#v", res)
	}
	if !res.MissingAny() {
		t.Fatalf("expected MissingAny true, got %#v", res)
	}
}
