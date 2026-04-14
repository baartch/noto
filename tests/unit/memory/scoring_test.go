package memory

import "testing"

func TestScoreCandidate_DefaultWhenEmpty(t *testing.T) {
	score := ScoreCandidate(ScoringInputs{})
	if score.Total != DefaultValueScore {
		t.Fatalf("expected default score %d, got %d", DefaultValueScore, score.Total)
	}
}

func TestScoreCandidate_HighValueNote(t *testing.T) {
	inputs := ScoringInputs{
		Content:    "User prefers dark mode for coding",
		Importance: 8,
		Evidence:   []string{"message"},
	}
	score := ScoreCandidate(inputs)
	if score.Total < MinValueScore {
		t.Fatalf("expected score >= %d, got %d", MinValueScore, score.Total)
	}
	if score.Total != 8 {
		t.Fatalf("expected score 8, got %d", score.Total)
	}
}
