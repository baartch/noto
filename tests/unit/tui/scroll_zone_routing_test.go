package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/store"
	"noto/internal/tui"
)

func newScrollModelWithHistory() tui.Model {
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
		[]string{"one", "two", "three", "four", "five"},
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	m.SetStartupConversationHistory([]*store.Message{{ID: "1", ConversationID: "c", Role: store.RoleAssistant, Content: "hello"}}, nil)
	return m
}

func TestMouseWheel_InputZoneNavigatesInputHistory(t *testing.T) {
	m := newScrollModelWithHistory()
	// textarea lines are near bottom; y=36 is inside input zone for height 40
	updated, _ := m.Update(tea.MouseWheelMsg{X: 2, Y: 36, Button: tea.MouseWheelUp})
	m = updated.(tui.Model)
	view := m.View().Content
	if !strings.Contains(view, "five") && !strings.Contains(view, "four") {
		t.Fatalf("expected input history navigation in input zone")
	}
}

func TestPageKeysAlwaysScrollMessages(t *testing.T) {
	m := newScrollModelWithHistory()
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	_ = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	_ = updated.(tui.Model)
}
