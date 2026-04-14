package memory

import "strings"

// ValueScore captures the scoring output for a note candidate.
type ValueScore struct {
	// Total is the aggregate score used for storage decisions.
	Total int
	// Importance weights long-term importance.
	Importance int
	// Specificity weights how concrete the note is.
	Specificity int
	// Usefulness weights future usefulness.
	Usefulness int
}

// ScoringInputs describes the inputs needed to score a note candidate.
type ScoringInputs struct {
	Content    string
	Importance int
	Evidence   []string
}

const (
	// MinValueScore is the minimum score required to store a note.
	MinValueScore = 6
	// DefaultValueScore is used when inputs are missing.
	DefaultValueScore = 5
)

// ScoreCandidate computes the score for a candidate note.
func ScoreCandidate(inputs ScoringInputs) ValueScore {
	content := strings.TrimSpace(inputs.Content)
	if content == "" {
		return ValueScore{Total: DefaultValueScore}
	}

	importance := inputs.Importance
	if importance <= 0 {
		importance = DefaultValueScore
	}

	specificity := 1
	if len(strings.Fields(content)) >= 4 {
		specificity = 2
	}

	usefulness := 1
	if len(inputs.Evidence) > 0 {
		usefulness = 2
	}

	total := max(importance, DefaultValueScore)

	return ValueScore{
		Total:       total,
		Importance:  importance,
		Specificity: specificity,
		Usefulness:  usefulness,
	}
}
