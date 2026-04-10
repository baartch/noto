package tui

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/textarea"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/suggest"
)

func TestSuggestionWindowingCentersCursor(t *testing.T) {
	m := Model{}
	m.suggestions = makeSuggestions(20)
	m.suggCursor = 10

	start, end := suggestionWindow(len(m.suggestions), m.suggCursor, 5)
	if start != 8 || end != 13 {
		t.Fatalf("expected window [8,13), got [%d,%d)", start, end)
	}
}

func TestSuggestionEngineUnlimited(t *testing.T) {
	registry := commands.NewRegistry()
	for i := range 12 {
		path := fmt.Sprintf("cmd %d", i)
		_ = registry.Register(&commands.Command{
			Path:        path,
			Usage:       path,
			Description: "desc",
			Handler: func(ctx *commands.ExecContext, args []string) error {
				return nil
			},
		})
	}

	engine := suggest.New(registry)
	got := engine.Suggest("")
	if len(got) != 12 {
		t.Fatalf("expected 12 suggestions, got %d", len(got))
	}
}

func TestSuggestionWindowingClampsStart(t *testing.T) {
	m := Model{}
	m.suggestions = makeSuggestions(5)
	m.suggCursor = 0

	start, end := suggestionWindow(len(m.suggestions), m.suggCursor, 3)
	if start != 0 || end != 3 {
		t.Fatalf("expected window [0,3), got [%d,%d)", start, end)
	}
}

func TestSuggestionWindowingClampsCursorToBounds(t *testing.T) {
	start, end := suggestionWindow(5, -2, 3)
	if start != 0 || end != 3 {
		t.Fatalf("expected window [0,3), got [%d,%d)", start, end)
	}
	start, end = suggestionWindow(5, 99, 3)
	if start != 2 || end != 5 {
		t.Fatalf("expected window [2,5), got [%d,%d)", start, end)
	}
}

func TestSuggestionWindowingClampsEnd(t *testing.T) {
	m := Model{}
	m.suggestions = makeSuggestions(5)
	m.suggCursor = 4

	start, end := suggestionWindow(len(m.suggestions), m.suggCursor, 3)
	if start != 2 || end != 5 {
		t.Fatalf("expected window [2,5), got [%d,%d)", start, end)
	}
}

func TestRefreshSuggestionsShowsOnSlash(t *testing.T) {
	m := newTestModelWithCommands("profile list")
	m.input.SetValue("/")
	m.refreshSuggestions()
	if len(m.suggestions) == 0 {
		t.Fatalf("expected suggestions for slash input")
	}
}

func TestRefreshSuggestionsClearsOnNonSlash(t *testing.T) {
	m := newTestModelWithCommands("profile list")
	m.suggestions = makeSuggestions(2)
	m.input.SetValue("hello")
	m.refreshSuggestions()
	if len(m.suggestions) != 0 {
		t.Fatalf("expected suggestions to clear for non-slash input")
	}
}

func TestRefreshSuggestionsFiltersByPrefix(t *testing.T) {
	m := newTestModelWithCommands("profile list", "prompt show")
	m.input.SetValue("/profile")
	m.refreshSuggestions()
	if len(m.suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(m.suggestions))
	}
	if m.suggestions[0].CommandPath != "profile list" {
		t.Fatalf("unexpected suggestion %q", m.suggestions[0].CommandPath)
	}
}

func makeSuggestions(count int) []suggest.Suggestion {
	items := make([]suggest.Suggestion, count)
	for i := range items {
		items[i] = suggest.Suggestion{CommandPath: "cmd"}
	}
	return items
}

func newTestModelWithCommands(paths ...string) Model {
	registry := commands.NewRegistry()
	for _, path := range paths {
		_ = registry.Register(&commands.Command{
			Path:        path,
			Usage:       path,
			Description: "desc",
			Handler: func(ctx *commands.ExecContext, args []string) error {
				return nil
			},
		})
	}
	return Model{
		input:      textarea.New(),
		dispatcher: chat.NewDispatcher(registry),
		execCtx:    &commands.ExecContext{},
	}
}
