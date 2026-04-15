package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/tui"
)

func TestFooterNoteIndicator_ShowsOnSave(t *testing.T) {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}

	model := tui.New(
		"Profile",
		"",
		"",
		"",
		"cache: n/a",
		"tokens: n/a",
		false,
		false,
		dispatcher,
		execCtx,
		nil,
		nil,
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
	)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(tui.Model)
	updated, cmd := m.Update(tui.NotesSaved(1, 0))
	_ = cmd
	m = updated.(tui.Model)
	view := m.View().Content
	if !strings.Contains(view, "note(s) saved") {
		t.Fatalf("expected footer note indicator, got: %s", view)
	}
}
