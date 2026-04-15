package memory_test

import (
	"testing"

	"noto/internal/memory"
)

func TestScoreCandidate_DefaultWhenEmpty(t *testing.T) {
	score := memory.ScoreCandidate(memory.ScoringInputs{})
	if score.Total != memory.DefaultValueScore {
		t.Fatalf("expected default score %d, got %d", memory.DefaultValueScore, score.Total)
	}
}

func TestScoreCandidate_HighValueNote(t *testing.T) {
	inputs := memory.ScoringInputs{
		Content:    "User prefers dark mode for coding",
		Importance: 8,
		Evidence:   []string{"message"},
	}
	score := memory.ScoreCandidate(inputs)
	if score.Total < memory.MinValueScore {
		t.Fatalf("expected score >= %d, got %d", memory.MinValueScore, score.Total)
	}
	if score.Total != 8 {
		t.Fatalf("expected score 8, got %d", score.Total)
	}
}
