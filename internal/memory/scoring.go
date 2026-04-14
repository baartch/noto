package memory

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
// Implementation is added in the user story phase.
func ScoreCandidate(_ ScoringInputs) ValueScore {
	return ValueScore{Total: DefaultValueScore}
}
