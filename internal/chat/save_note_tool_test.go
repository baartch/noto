package chat

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"noto/internal/memory"
	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
)

func newSaveNoteTestSession(t *testing.T, adapter *chatRecordingAdapter, toolsEnabled bool, onNotes NotesCallback) (*Session, *store.MemoryNoteRepo) {
	t.Helper()
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	convRepo := store.NewConversationRepo(db)
	if err := convRepo.Create(ctx, &store.Conversation{ID: "conv", ProfileID: "p", Status: store.ConversationActive}); err != nil {
		t.Fatal(err)
	}
	noteRepo := store.NewMemoryNoteRepo(db)
	session := &Session{
		profileID: "p", conversationID: "conv", systemPrompt: "system instructions",
		convRepo: store.NewConversationRepo(db), msgRepo: store.NewMessageRepo(db), noteRepo: noteRepo,
		adapter: adapter, extractor: memory.NewExtractor(noteRepo, adapter, store.NewContextCacheRepo(db)),
		cacheRepo: store.NewContextCacheRepo(db), db: db, logger: observe.NewJSONLogger(io.Discard),
		history:    []*store.Message{{ID: "old-user", Role: store.RoleUser, Content: "hi"}},
		backupStop: make(chan struct{}), pendingDone: make(chan struct{}),
		toolsEnabled: toolsEnabled,
		onNotes:      onNotes,
	}
	return session, noteRepo
}

func TestSendExecutesSaveNoteToolCallAndPersistsNote(t *testing.T) {
	adapter := &chatRecordingAdapter{responses: []*provider.CompletionResponse{
		{ToolCalls: []provider.ToolCall{{ID: "call_1", CallID: "call_1", Name: "save_note", Arguments: `{"content":"User's daughter is named Anna","category":"fact","importance":8}`}}},
		{Content: "Done.", Model: "chat"},
	}}
	var mu sync.Mutex
	var notesCalls [][2]int
	onNotes := func(saved, updated int) {
		mu.Lock()
		notesCalls = append(notesCalls, [2]int{saved, updated})
		mu.Unlock()
	}
	session, noteRepo := newSaveNoteTestSession(t, adapter, true, onNotes)
	ctx := context.Background()

	result, err := session.Send(ctx, "Please remember: my daughter is named Anna")
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if result == nil || result.Reply != "Done." {
		t.Fatalf("Send() = %#v, want reply exactly %q (no injected acknowledgment)", result, "Done.")
	}

	toolResultSeen := false
	toolCallSeen := false
	requests := adapter.snapshot()
	if len(requests) < 2 {
		t.Fatalf("main provider call count = %d, want >= 2 (tool follow-up)", len(requests))
	}
	for _, msg := range requests[1].Messages {
		if msg.Role == "assistant" && msg.ToolCallName == "save_note" && msg.ToolCallID == "call_1" {
			toolCallSeen = true
		}
		if msg.Role == "tool" && msg.ToolCallID == "call_1" && strings.Contains(msg.Content, `"saved":true`) {
			toolResultSeen = true
		}
	}
	if !toolCallSeen || !toolResultSeen {
		t.Fatalf("follow-up request missing tool call/tool result messages: %#v", requests[1].Messages)
	}

	mu.Lock()
	sawSaved := false
	for _, c := range notesCalls {
		if c == [2]int{1, 0} {
			sawSaved = true
		}
	}
	mu.Unlock()
	if !sawSaved {
		t.Fatalf("onNotes not called with (1,0): %v", notesCalls)
	}

	session.Close(ctx)
	stored, err := noteRepo.ListByProfile(ctx, "p")
	if err != nil || len(stored) != 1 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 1", len(stored), err)
	}
	note := stored[0]
	if note.Category != store.CategoryFact || note.Importance != 8 || note.ConversationID != "conv" {
		t.Fatalf("note fields wrong: %#v", note)
	}
	msgs, err := session.msgRepo.ListByConversation(ctx, "conv")
	if err != nil || len(msgs) == 0 {
		t.Fatalf("ListByConversation() = %d, %v", len(msgs), err)
	}
	var userMsgID string
	for _, m := range msgs {
		if m.Role == store.RoleUser {
			userMsgID = m.ID
			break
		}
	}
	if userMsgID == "" || !strings.Contains(note.SourceMessageIDs, `"`+userMsgID+`"`) {
		t.Fatalf("note not attributed to current user message %q: %s", userMsgID, note.SourceMessageIDs)
	}
}

func TestSendSaveNoteToolRejectsInvalidArgs(t *testing.T) {
	adapter := &chatRecordingAdapter{responses: []*provider.CompletionResponse{
		{ToolCalls: []provider.ToolCall{{ID: "call_1", CallID: "call_1", Name: "save_note", Arguments: `{"content":""}`}}},
		{Content: "Cannot save that.", Model: "chat"},
	}}
	session, noteRepo := newSaveNoteTestSession(t, adapter, true, nil)
	ctx := context.Background()

	result, err := session.Send(ctx, "remember nothing")
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if result == nil || result.Reply != "Cannot save that." {
		t.Fatalf("Send() = %#v, want turn to complete", result)
	}
	requests := adapter.snapshot()
	if len(requests) < 2 {
		t.Fatalf("main provider call count = %d, want >= 2", len(requests))
	}
	toolResultSeen := false
	for _, msg := range requests[1].Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call_1" && strings.Contains(msg.Content, `"error"`) {
			toolResultSeen = true
		}
	}
	if !toolResultSeen {
		t.Fatalf("follow-up missing error tool result: %#v", requests[1].Messages)
	}
	session.Close(ctx)
	stored, err := noteRepo.ListByProfile(ctx, "p")
	if err != nil || len(stored) != 0 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 0", len(stored), err)
	}
}

func TestSendOmitsSaveNoteToolWhenToolsDisabled(t *testing.T) {
	adapter := &chatRecordingAdapter{responses: []*provider.CompletionResponse{
		{ToolCalls: []provider.ToolCall{{ID: "call_1", CallID: "call_1", Name: "save_note", Arguments: `{"content":"User likes chess"}`}}},
	}}
	session, noteRepo := newSaveNoteTestSession(t, adapter, false, nil)
	ctx := context.Background()

	result, err := session.Send(ctx, "remember that I like chess")
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if result == nil {
		t.Fatal("Send() = nil result")
	}
	requests := adapter.snapshot()
	if len(requests) == 0 || len(requests[0].Tools) != 0 {
		t.Fatalf("tools were exposed while disabled: %#v", requests)
	}
	session.Close(ctx)
	stored, err := noteRepo.ListByProfile(ctx, "p")
	if err != nil || len(stored) != 0 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 0", len(stored), err)
	}
}

func TestChatToolDefinitionsIncludeSaveNote(t *testing.T) {
	defs := chatToolDefinitions()
	if len(defs) != 3 {
		t.Fatalf("chatToolDefinitions() = %d tools, want 3", len(defs))
	}
	var saveNote *provider.ToolDefinition
	for i := range defs {
		if defs[i].Name == "save_note" {
			saveNote = &defs[i]
			break
		}
	}
	if saveNote == nil {
		t.Fatalf("save_note missing from chatToolDefinitions(): %#v", defs)
	}
	if !strings.Contains(strings.ToLower(saveNote.Description), "do not mention") {
		t.Fatalf("save_note description lacks no-acknowledgment instruction: %q", saveNote.Description)
	}
	schema, err := json.Marshal(saveNote.Parameters)
	if err != nil {
		t.Fatal(err)
	}
	s := string(schema)
	for _, want := range []string{
		`"required":["content"]`,
		`"enum":["fact","progress","blocker","action_item","other"]`,
		`"minimum":1`,
		`"maximum":10`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("save_note schema missing %s: %s", want, s)
		}
	}
}
