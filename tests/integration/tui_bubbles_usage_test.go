package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/tui"
)

func TestTUIModel_UsesBubblesComponents(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	model := tui.New(
		"Profile",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		dispatcher,
		execCtx,
		nil,
		nil,
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("expected Update to return tui.Model")
	}

	if m.InputPlaceholder() == "" {
		t.Fatalf("expected textarea placeholder to be set")
	}
	if m.ViewportHeight() == 0 {
		t.Fatalf("expected viewport to be initialized")
	}

	view := m.View().Content
	if !strings.Contains(view, "ctrl+d") {
		t.Fatalf("expected help bindings to be rendered in footer")
	}
}
