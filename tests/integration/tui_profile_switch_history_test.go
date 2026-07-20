package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/store"
	"noto/internal/tui"
)

func newProfileSwitchModel() tui.Model {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}
	m := tui.New(
		"Profile A", "", "", "", "cache: n/a", "tokens: n/a", false, false,
		dispatcher, execCtx,
		nil, nil, nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil,
		func(string) tea.Cmd { return nil },
		nil,
		func(string) error { return nil },
		func(string) error { return nil },
		nil, false,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(tui.Model)
}

func profileMessages(prefix string, n int) []*store.Message {
	base := time.Now().Add(-time.Hour)
	msgs := make([]*store.Message, 0, n)
	for i := 1; i <= n; i++ {
		role := store.RoleUser
		if i%2 == 0 {
			role = store.RoleAssistant
		}
		msgs = append(msgs, &store.Message{
			ID:             fmt.Sprintf("%s-%d", prefix, i),
			ConversationID: prefix,
			Role:           role,
			Content:        fmt.Sprintf("%s-msg-%d", prefix, i),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		})
	}
	return msgs
}

func TestProfileSwitch_ReplacesVisibleConversationHistory(t *testing.T) {
	m := newProfileSwitchModel()
	m.SetStartupConversationHistory(profileMessages("A", 4), nil)
	if !strings.Contains(m.View().Content, "A-msg-1") {
		t.Fatalf("expected initial profile A history")
	}

	msg := tui.ProfileSwitched(
		"Profile B", "", "", "", "cache: n/a", "tokens: n/a", false, false,
		nil, nil, nil,
		func(string) error { return nil },
		func(string) error { return nil },
		func(string) error { return nil },
		tui.DefaultSettingsMenu(),
		nil,
		profileMessages("B", 2),
		nil,
		false,
	)
	updated, _ := m.Update(msg)
	m = updated.(tui.Model)
	view := m.View().Content

	if strings.Contains(view, "A-msg-1") {
		t.Fatalf("expected old profile messages to be replaced")
	}
	if !strings.Contains(view, "B-msg-1") || !strings.Contains(view, "B-msg-2") {
		t.Fatalf("expected switched profile messages in view")
	}
}
