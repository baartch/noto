package memory

import "testing"

func TestEvaluateCandidate_TrimsContent(t *testing.T) {
	candidate := EvaluateCandidate("  user prefers espresso  ", 7, []string{"msg"})
	if candidate.Content != "user prefers espresso" {
		t.Fatalf("expected trimmed content, got %q", candidate.Content)
	}
	if candidate.ValueScore.Total < MinValueScore {
		t.Fatalf("expected score >= %d, got %d", MinValueScore, candidate.ValueScore.Total)
	}
}
