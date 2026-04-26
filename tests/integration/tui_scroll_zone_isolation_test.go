package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/store"
	"noto/internal/tui"
)

func newIsolationModel() tui.Model {
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
		[]string{"u1", "u2", "u3", "u4", "u5", "u6"},
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	m.SetStartupConversationHistory([]*store.Message{{ID: "1", ConversationID: "c", Role: store.RoleAssistant, Content: "asst"}}, nil)
	return m
}

func TestWheelInMessagesZone_DoesNotMutateInputHistory(t *testing.T) {
	m := newIsolationModel()
	before := m.View().Content
	updated, _ := m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelUp})
	m = updated.(tui.Model)
	after := m.View().Content
	if strings.Count(after, "u6") != strings.Count(before, "u6") {
		t.Fatalf("expected input history text unaffected by messages-zone wheel")
	}
}

func TestWheelInInputZone_DoesNotMoveMessagesHistory(t *testing.T) {
	m := newIsolationModel()
	before := m.View().Content
	updated, _ := m.Update(tea.MouseWheelMsg{X: 2, Y: 36, Button: tea.MouseWheelUp})
	m = updated.(tui.Model)
	after := m.View().Content
	if !strings.Contains(after, "asst") || !strings.Contains(before, "asst") {
		t.Fatalf("expected messages area to remain present")
	}
}
