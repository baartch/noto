package memory

import "strings"

// NoteCandidate represents a provisional note extracted from a chat turn.
type NoteCandidate struct {
	Content     string
	ValueScore  ValueScore
	DuplicateOf string
	Evidence    []string
}

// EvaluateCandidate normalizes and scores a raw candidate string.
// Implementation is added in the user story phase.
func EvaluateCandidate(content string, evidence []string) NoteCandidate {
	clean := strings.TrimSpace(content)
	return NoteCandidate{
		Content:    clean,
		ValueScore: ScoreCandidate(ScoringInputs{Content: clean, Evidence: evidence}),
		Evidence:   evidence,
	}
}
