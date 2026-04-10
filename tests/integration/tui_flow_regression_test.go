package integration

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/tui"
)

func TestTUIModel_HandlesWindowResize(t *testing.T) {
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

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if _, ok := updated.(tui.Model); !ok {
		t.Fatalf("expected Update to return tui.Model")
	}
}
