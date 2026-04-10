package tui

import (
	"strings"
	"testing"
)

func TestRenderSuggestionsWindowScrollsToCursor(t *testing.T) {
	m := Model{}
	m.suggestions = makeSuggestions(20)
	m.suggCursor = 18

	out := m.renderSuggestions(5)
	if !strings.Contains(out, "cmd") {
		t.Fatalf("expected rendered suggestions to include command text")
	}
	if !strings.Contains(out, "… 15 more") {
		t.Fatalf("expected overflow indicator in rendered suggestions")
	}
}
