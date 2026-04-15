package memory_test

import (
	"testing"

	"noto/internal/memory"
)

func TestEvaluateCandidate_TrimsContent(t *testing.T) {
	candidate := memory.EvaluateCandidate("  user prefers espresso  ", 7, []string{"msg"})
	if candidate.Content != "user prefers espresso" {
		t.Fatalf("expected trimmed content, got %q", candidate.Content)
	}
	if candidate.ValueScore.Total < memory.MinValueScore {
		t.Fatalf("expected score >= %d, got %d", memory.MinValueScore, candidate.ValueScore.Total)
	}
}
