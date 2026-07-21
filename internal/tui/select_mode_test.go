package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/store"
)

func newSelectTestModel(msgs ...string) Model {
	registry := commands.NewRegistry()
	dispatcher := chat.NewDispatcher(registry)
	execCtx := &commands.ExecContext{}
	m := New(
		"Profile", "test-model", "", "", "cache: n/a", "tokens: n/a", false, false,
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
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(Model)

	// Populate messages directly.
	now := time.Now()
	for _, content := range msgs {
		m.messages = append(m.messages, chatMessage{
			role:      "assistant",
			content:   content,
			timestamp: now,
		})
	}
	m.syncViewport()
	return m
}

func TestSelectMode_ToggleOnOff(t *testing.T) {
	m := newSelectTestModel("hello")
	// Enter select mode.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	if !m.selectMode {
		t.Fatal("expected selectMode=true after ctrl+a")
	}

	// Exit select mode.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	if m.selectMode {
		t.Fatal("expected selectMode=false after second ctrl+a")
	}
}

func TestSelectMode_EscExits(t *testing.T) {
	m := newSelectTestModel("hello")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	if !m.selectMode {
		t.Fatal("expected selectMode=true")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.selectMode {
		t.Fatal("expected selectMode=false after Esc")
	}
}

func TestSelectMode_CursorMovement(t *testing.T) {
	m := newSelectTestModel("msg1", "msg2", "msg3")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	if m.selectCursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.selectCursor)
	}

	// Move down.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.selectCursor != 1 {
		t.Fatalf("expected cursor 1 after down, got %d", m.selectCursor)
	}

	// Move down again.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.selectCursor != 2 {
		t.Fatalf("expected cursor 2 after down, got %d", m.selectCursor)
	}

	// Move down again — should clamp.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.selectCursor != 2 {
		t.Fatalf("expected cursor 2 (clamped), got %d", m.selectCursor)
	}

	// Move up.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(Model)
	if m.selectCursor != 1 {
		t.Fatalf("expected cursor 1 after up, got %d", m.selectCursor)
	}
}

func TestSelectMode_CursorBoundsClamp(t *testing.T) {
	m := newSelectTestModel("only-one")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	// Up from 0 should stay at 0.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(Model)
	if m.selectCursor != 0 {
		t.Fatalf("expected cursor 0 (clamped up), got %d", m.selectCursor)
	}
}

func TestSelectMode_FocusSwitch(t *testing.T) {
	m := newSelectTestModel("msg1", "msg2")
	m.sidebar = newSidebar(36, nil, nil, "")
	m.sidebar.open = true

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	if m.selectFocus != selectFocusChat {
		t.Fatal("expected initial focus on chat")
	}

	// Alt+Right to switch to sidebar.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt})
	m = updated.(Model)
	if m.selectFocus != selectFocusSidebar {
		t.Fatal("expected focus on sidebar after alt+right")
	}

	// Alt+Left to switch back to chat.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	m = updated.(Model)
	if m.selectFocus != selectFocusChat {
		t.Fatal("expected focus on chat after alt+left")
	}
}

func TestSelectMode_FocusSwitch_SidebarClosed(t *testing.T) {
	m := newSelectTestModel("msg1")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	// Alt+Right with sidebar nil should not panic and stay on chat.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt})
	m = updated.(Model)
	if m.selectFocus != selectFocusChat {
		t.Fatal("expected focus to remain on chat when sidebar is nil")
	}
}

func TestSelectMode_ChatEntries(t *testing.T) {
	m := newSelectTestModel("hello", "world")
	entries := m.chatEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0] != "hello" || entries[1] != "world" {
		t.Fatalf("unexpected entries: %v", entries)
	}
}

func TestSelectMode_SidebarEntries_Notes(t *testing.T) {
	m := newSelectTestModel()
	m.sidebar = newSidebar(36, nil, nil, "")
	m.sidebar.open = true
	m.sidebar.notes = []*store.MemoryNote{
		{ID: "n1", Content: "note one", Category: store.CategoryFact, Importance: 5, CreatedAt: time.Now()},
		{ID: "n2", Content: "note two", Category: store.CategoryFact, Importance: 3, CreatedAt: time.Now()},
	}

	entries := m.sidebarEntries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0] != "note one" || entries[1] != "note two" {
		t.Fatalf("unexpected entries: %v", entries)
	}
}

func TestSelectMode_SidebarEntries_Summaries(t *testing.T) {
	m := newSelectTestModel()
	m.sidebar = newSidebar(36, nil, nil, "")
	m.sidebar.open = true
	m.sidebar.activeTab = sidebarTabWeekly
	m.sidebar.weekly = []*store.MemorySummary{
		{ID: "s1", Content: "summary one", PeriodKey: "2026-W27", PeriodStart: time.Now(), PeriodEnd: time.Now()},
	}

	entries := m.sidebarEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0] != "summary one" {
		t.Fatalf("unexpected entry: %v", entries)
	}
}

func TestSelectMode_SidebarEntries_NilSidebar(t *testing.T) {
	m := newSelectTestModel()
	m.sidebar = nil
	entries := m.sidebarEntries()
	if entries != nil {
		t.Fatalf("expected nil entries, got %v", entries)
	}
}

func TestSelectMode_FooterShowsBadge(t *testing.T) {
	m := newSelectTestModel("msg")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	footer := m.renderFooter()
	if !strings.Contains(footer, "[SELECT]") {
		t.Fatalf("expected [SELECT] in footer, got: %s", footer)
	}
}

func TestSelectMode_FooterNormalNoBadge(t *testing.T) {
	m := newSelectTestModel("msg")
	footer := m.renderFooter()
	if strings.Contains(footer, "[SELECT]") {
		t.Fatalf("unexpected [SELECT] in normal footer: %s", footer)
	}
}

func TestSelectMode_RenderHighlight(t *testing.T) {
	m := newSelectTestModel("hello world")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	rendered := m.renderHistory()
	// The selected entry should contain the selectBgStyle background color (xterm 18).
	if !strings.Contains(rendered, "48;5;18") {
		t.Fatalf("expected selection highlight background in rendered history, got:\n%s", rendered)
	}
}

func TestSelectMode_RenderNoHighlightWhenNormal(t *testing.T) {
	m := newSelectTestModel("hello world")
	rendered := m.renderHistory()
	if strings.Contains(rendered, "48;5;18") {
		t.Fatalf("unexpected selection highlight in normal mode")
	}
}

func TestSelectMode_CtrlHTogglesHelp(t *testing.T) {
	m := newSelectTestModel("msg")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	if !m.selectMode {
		t.Fatal("expected selectMode=true")
	}

	// ctrl+h should toggle help.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	m = updated.(Model)
	if !m.help.ShowAll {
		t.Fatal("expected help.ShowAll=true after ctrl+h in select mode")
	}

	// ctrl+h again should toggle off.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	m = updated.(Model)
	if m.help.ShowAll {
		t.Fatal("expected help.ShowAll=false after second ctrl+h")
	}
}

func TestSelectMode_NormalKeyPressIgnored(t *testing.T) {
	m := newSelectTestModel("msg1", "msg2")
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	// Any regular key should be consumed (not cause side effects).
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if !m.selectMode {
		t.Fatal("expected selectMode to remain true after consuming key")
	}
	if m.selectCursor != 0 {
		t.Fatal("expected cursor to remain 0")
	}
}

func TestSelectMode_CopyCmdNilOnEmpty(t *testing.T) {
	m := newSelectTestModel()
	cmd := m.copySelectionToClipboard()
	if cmd != nil {
		t.Fatal("expected nil cmd when no entries to copy")
	}
}

func TestSelectMode_SyncSelectState(t *testing.T) {
	m := newSelectTestModel("msg1")
	m.sidebar = newSidebar(36, nil, nil, "")
	m.sidebar.open = true

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	m.selectFocus = selectFocusSidebar
	m.syncSelectState()

	if !m.sidebar.selectActive {
		t.Fatal("expected sidebar.selectActive=true when focus is sidebar")
	}
	if m.sidebar.selectCursor != 0 {
		t.Fatalf("expected sidebar.selectCursor=0, got %d", m.sidebar.selectCursor)
	}
}

func TestSelectMode_SidebarHighlight(t *testing.T) {
	s := newSidebar(40, nil, nil, "")
	s.open = true
	s.width = 40
	s.selectActive = true
	s.selectCursor = 0
	s.notes = []*store.MemoryNote{
		{ID: "n1", Category: store.CategoryFact, Content: "first note", Importance: 5, CreatedAt: time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)},
		{ID: "n2", Category: store.CategoryFact, Content: "second note", Importance: 3, CreatedAt: time.Date(2026, 7, 9, 15, 0, 0, 0, time.UTC)},
	}

	content := s.renderContent()
	// Notes are rendered in reverse: n2 first (displayed position 0), n1 second (displayed position 1).
	// selectCursor=0 means the first displayed entry (n2) should be highlighted.
	if !strings.Contains(content, "48;5;18") {
		t.Fatalf("expected selection highlight in sidebar content, got:\n%s", content)
	}
}

func TestSelectMode_SidebarHighlightDisabled(t *testing.T) {
	s := newSidebar(40, nil, nil, "")
	s.open = true
	s.width = 40
	s.selectActive = false
	s.notes = []*store.MemoryNote{
		{ID: "n1", Category: store.CategoryFact, Content: "a note", Importance: 5, CreatedAt: time.Now()},
	}

	content := s.renderContent()
	if strings.Contains(content, "48;5;18") {
		t.Fatal("unexpected selection highlight when selectActive=false")
	}
}

func TestSelectMode_BubbleSelectedParam(t *testing.T) {
	ts := time.Date(2026, 7, 9, 14, 0, 0, 0, time.UTC)
	normal := renderUserBubble("test", "You", ts, 80, false)
	selected := renderUserBubble("test", "You", ts, 80, true)

	if normal == selected {
		t.Fatal("expected selected rendering to differ from normal")
	}
	if !strings.Contains(selected, "48;5;18") {
		t.Fatal("expected selection background in selected bubble")
	}
	if strings.Contains(normal, "48;5;18") {
		t.Fatal("unexpected selection background in normal bubble")
	}
}

func TestSelectMode_AssistantBubbleSelectedParam(t *testing.T) {
	ts := time.Date(2026, 7, 9, 14, 0, 0, 0, time.UTC)
	normal := renderAssistantBubble("test content", "model", ts, 80, false)
	selected := renderAssistantBubble("test content", "model", ts, 80, true)

	if normal == selected {
		t.Fatal("expected selected rendering to differ from normal")
	}
	if !strings.Contains(selected, "48;5;18") {
		t.Fatal("expected selection background in selected assistant bubble")
	}
}

func TestSelectMode_MoveSelectCursor(t *testing.T) {
	m := newSelectTestModel("a", "b", "c")
	m.selectMode = true
	m.selectCursor = 1

	m.moveSelectCursor(-1)
	if m.selectCursor != 0 {
		t.Fatalf("expected cursor 0 after -1, got %d", m.selectCursor)
	}

	m.moveSelectCursor(1)
	if m.selectCursor != 1 {
		t.Fatalf("expected cursor 1 after +1, got %d", m.selectCursor)
	}

	m.moveSelectCursor(10)
	if m.selectCursor != 2 {
		t.Fatalf("expected cursor 2 (clamped), got %d", m.selectCursor)
	}

	m.moveSelectCursor(-10)
	if m.selectCursor != 0 {
		t.Fatalf("expected cursor 0 (clamped), got %d", m.selectCursor)
	}
}
