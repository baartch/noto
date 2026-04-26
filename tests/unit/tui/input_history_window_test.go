package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/tui"
)

func newInputHistoryModel(hist []string) tui.Model {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}
	m := tui.New(
		"Profile", "", "", "", "cache: n/a", "tokens: n/a", false, false,
		dispatcher, execCtx,
		nil, nil, nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		hist,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(tui.Model)
}

func TestInputHistory_FirstScrollLoadsWindow(t *testing.T) {
	hist := []string{"a", "b", "c", "d", "e", "f"}
	m := newInputHistoryModel(hist)
	updated, _ := m.Update(tea.MouseWheelMsg{X: 2, Y: 36, Button: tea.MouseWheelUp})
	_ = updated.(tui.Model)
}

func TestInputHistory_ClearsAfterSend(t *testing.T) {
	hist := []string{"a", "b", "c", "d", "e", "f"}
	m := newInputHistoryModel(hist)
	updated, _ := m.Update(tea.MouseWheelMsg{X: 2, Y: 36, Button: tea.MouseWheelUp})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Text: "x", Code: 'x'})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = updated.(tui.Model)
}
