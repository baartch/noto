package integration

import (
	"context"
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

func newStartupIntegrationModel() tui.Model {
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
		nil, false,
	)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(tui.Model)
}

func makeStartupMessages(n int) []*store.Message {
	base := time.Now().Add(-2 * time.Hour)
	msgs := make([]*store.Message, 0, n)
	for i := 1; i <= n; i++ {
		role := store.RoleUser
		if i%2 == 0 {
			role = store.RoleAssistant
		}
		msgs = append(msgs, &store.Message{
			ID:             fmt.Sprintf("startup-%02d", i),
			ConversationID: "conv-startup",
			Role:           role,
			Content:        fmt.Sprintf("startup-msg-%02d", i),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		})
	}
	return msgs
}

func TestTUIStartup_LoadsLatestTenMessages(t *testing.T) {
	m := newStartupIntegrationModel()
	m.SetStartupConversationHistory(makeStartupMessages(14), nil)
	view := m.View().Content

	if strings.Contains(view, "startup-msg-01") || strings.Contains(view, "startup-msg-02") || strings.Contains(view, "startup-msg-03") || strings.Contains(view, "startup-msg-04") {
		t.Fatalf("expected only latest ten messages to be shown")
	}
	for i := 5; i <= 14; i++ {
		if !strings.Contains(view, fmt.Sprintf("startup-msg-%02d", i)) {
			t.Fatalf("expected startup-msg-%02d in view", i)
		}
	}
}

func TestTUIProfileSwitch_UsesProvidedStartupMessages(t *testing.T) {
	m := newStartupIntegrationModel()
	msg := tui.ProfileSwitched(
		"Profile B", "", "", "", "cache: n/a", "tokens: n/a", false, false,
		nil, nil, nil,
		func(string) error { return nil },
		func(string) error { return nil },
		func(string) error { return nil },
		tui.DefaultSettingsMenu(),
		nil,
		makeStartupMessages(3),
		nil,
		false,
	)
	updated, _ := m.Update(msg)
	m = updated.(tui.Model)
	view := m.View().Content

	if !strings.Contains(view, "startup-msg-01") || !strings.Contains(view, "startup-msg-02") || !strings.Contains(view, "startup-msg-03") {
		t.Fatalf("expected startup messages after profile switch")
	}
}

func TestTUIStartup_ReadFailureIsNonFatal(t *testing.T) {
	m := newStartupIntegrationModel()
	m.SetStartupConversationHistory(nil, context.DeadlineExceeded)
	view := m.View().Content
	if !strings.Contains(view, "deadline exceeded") {
		t.Fatalf("expected non-fatal load error in view")
	}
	if !strings.Contains(view, "No messages yet") {
		t.Fatalf("expected app to remain usable with empty state")
	}
}
