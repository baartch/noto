package memory

import "strings"

// NoteCandidate represents a provisional note extracted from a chat turn.
type NoteCandidate struct {
	Content     string
	Category    string
	Importance  int
	ValueScore  ValueScore
	DuplicateOf string
	Evidence    []string
}

// EvaluateCandidate normalizes and scores a raw candidate string.
func EvaluateCandidate(content string, importance int, evidence []string) NoteCandidate {
	clean := strings.TrimSpace(content)
	return NoteCandidate{
		Content:    clean,
		Importance: importance,
		ValueScore: ScoreCandidate(ScoringInputs{Content: clean, Importance: importance, Evidence: evidence}),
		Evidence:   evidence,
	}
}
