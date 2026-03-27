package suggest

import (
	"sort"
	"strings"

	"noto/internal/commands"
)

const (
	// MaxSuggestions is the maximum number of suggestions returned for a given prefix.
	MaxSuggestions = 8
)

// Suggestion is a ranked command suggestion.
type Suggestion struct {
	// CommandPath is the canonical hierarchical path of the suggested command.
	CommandPath string

	// Hint is the short description shown alongside the suggestion.
	Hint string

	// Rank is the ordering score; lower is better.
	Rank int
}

// Engine provides ranked slash command suggestions from a command registry.
type Engine struct {
	registry *commands.Registry
}

// New creates a suggestion engine backed by the given registry.
func New(registry *commands.Registry) *Engine {
	return &Engine{registry: registry}
}

// Suggest returns ranked suggestions for the given input prefix.
// prefix should be the raw text after the '/', e.g. "pro", "profile li".
// Returns an empty slice when no matches are found.
func (e *Engine) Suggest(prefix string) []Suggestion {
	matches := e.registry.PrefixMatches(prefix)
	suggestions := make([]Suggestion, 0, len(matches))

	lower := strings.ToLower(prefix)
	for _, cmd := range matches {
		rank := scoreMatch(strings.ToLower(cmd.Path), lower)
		suggestions = append(suggestions, Suggestion{
			CommandPath: cmd.Path,
			Hint:        cmd.Description,
			Rank:        rank,
		})
	}

	// Sort by rank ascending, then alphabetically for stability.
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Rank != suggestions[j].Rank {
			return suggestions[i].Rank < suggestions[j].Rank
		}
		return suggestions[i].CommandPath < suggestions[j].CommandPath
	})

	if len(suggestions) > MaxSuggestions {
		suggestions = suggestions[:MaxSuggestions]
	}
	return suggestions
}

// scoreMatch returns a rank score for a candidate path given the typed prefix.
// Lower scores are better:
//   0 = exact match
//   1 = exact prefix of first segment
//   2 = prefix match anywhere in path
func scoreMatch(path, prefix string) int {
	if path == prefix {
		return 0
	}
	if strings.HasPrefix(path, prefix) {
		// Check if the prefix ends at a word boundary (space or end of path).
		rest := path[len(prefix):]
		if rest == "" || rest[0] == ' ' {
			return 1
		}
		return 2
	}
	return 3
}

// TopSuggestionsText returns a formatted list of suggestions as human-readable strings
// suitable for display in the TUI or CLI. Each entry is "/path  — hint".
func TopSuggestionsText(suggestions []Suggestion) []string {
	lines := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		lines = append(lines, "/"+s.CommandPath+"  — "+s.Hint)
	}
	return lines
}
