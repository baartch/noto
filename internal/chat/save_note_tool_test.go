package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
)

func newSaveNoteTestSession(t *testing.T, adapter provider.Adapter, toolsEnabled bool, onNotes NotesCallback) (*Session, *store.MemoryNoteRepo) {
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

func TestSendToolLoopRunsMultipleRounds(t *testing.T) {
	adapter := &chatRecordingAdapter{responses: []*provider.CompletionResponse{
		{ToolCalls: []provider.ToolCall{{ID: "call_1", CallID: "call_1", Name: "search_memory_keywords", Arguments: `{"query":"ship date"}`}}},
		{ToolCalls: []provider.ToolCall{{ID: "call_2", CallID: "call_2", Name: "save_note", Arguments: `{"content":"Ship date is Nov 5","category":"fact","importance":7}`}}},
		{Content: "All done.", Model: "chat"},
	}}
	session, noteRepo := newSaveNoteTestSession(t, adapter, true, nil)
	ctx := context.Background()

	result, err := session.Send(ctx, "When do we ship?")
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if result == nil || result.Reply != "All done." {
		t.Fatalf("Send() = %#v, want reply %q", result, "All done.")
	}
	requests := adapter.snapshot()
	if len(requests) < 3 {
		t.Fatalf("main provider call count = %d, want >= 3 (search round, save round, final)", len(requests))
	}
	if len(requests[0].Tools) != 3 {
		t.Fatalf("initial request tools = %d, want 3", len(requests[0].Tools))
	}
	sawSearch, sawSave := false, false
	for _, msg := range requests[1].Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call_1" {
			sawSearch = true
		}
	}
	for _, msg := range requests[2].Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call_2" && strings.Contains(msg.Content, `"saved":true`) {
			sawSave = true
		}
	}
	if !sawSearch || !sawSave {
		t.Fatalf("missing tool results across rounds: search=%t save=%t", sawSearch, sawSave)
	}
	session.Close(ctx)
	stored, err := noteRepo.ListByProfile(ctx, "p")
	if err != nil || len(stored) != 1 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 1", len(stored), err)
	}
	if stored[0].Content != "Ship date is Nov 5" {
		t.Fatalf("note content = %q", stored[0].Content)
	}
}

// saveNoteLoopAdapter answers every completion with another save_note tool
// call carrying a unique low-overlap content, so the loop bound is what stops
// the turn.
type saveNoteLoopAdapter struct {
	mu       sync.Mutex
	requests []provider.CompletionRequest
	calls    int
}

func (a *saveNoteLoopAdapter) Complete(_ context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, req)
	a.calls++
	if len(req.Tools) == 0 {
		// Forced text-only follow-up: no tools offered, answer in text.
		return &provider.CompletionResponse{Content: "Stopped after tool rounds."}, nil
	}
	callID := fmt.Sprintf("call_%d", a.calls)
	return &provider.CompletionResponse{
		ToolCalls: []provider.ToolCall{{
			ID: callID, CallID: callID, Name: "save_note",
			Arguments: fmt.Sprintf(`{"content":"fact %d"}`, a.calls),
		}},
	}, nil
}

func (*saveNoteLoopAdapter) Embed(context.Context, provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return nil, nil
}
func (*saveNoteLoopAdapter) ProviderType() string { return "test" }

func TestSendToolLoopStopsAtRoundLimit(t *testing.T) {
	adapter := &saveNoteLoopAdapter{}
	session, _ := newSaveNoteTestSession(t, adapter, true, nil)
	ctx := context.Background()

	result, err := session.Send(ctx, "loop forever")
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if result == nil {
		t.Fatal("Send() = nil result")
	}
	if result.Reply != "Stopped after tool rounds." {
		t.Fatalf("reply = %q, want the forced text-only answer, not a blank reply", result.Reply)
	}
	adapter.mu.Lock()
	calls := len(adapter.requests)
	forcedNoTools := len(adapter.requests[len(adapter.requests)-1].Tools) == 0
	adapter.mu.Unlock()
	if calls != maxToolRounds+2 {
		t.Fatalf("provider call count = %d, want %d (initial + %d rounds + forced reply)", calls, maxToolRounds+2, maxToolRounds)
	}
	if !forcedNoTools {
		t.Fatal("forced reply request still offered tools")
	}
}

func TestSendToolSaveAndExtractionCoexist(t *testing.T) {
	mainAdapter := &chatRecordingAdapter{responses: []*provider.CompletionResponse{
		{ToolCalls: []provider.ToolCall{{ID: "call_1", CallID: "call_1", Name: "save_note", Arguments: `{"content":"User lives in Lisbon","category":"fact","importance":7}`}}},
		{Content: "Noted.", Model: "chat"},
	}}
	extractAdapter := &chatRecordingAdapter{callDone: make(chan struct{}, 4), responses: []*provider.CompletionResponse{
		{Content: `{"has_new_info":true,"confidence":0.9,"notes":[{"action":"add","category":"fact","content":"User lives in Lisbon","importance":7}]}`},
	}}
	session, noteRepo := newSaveNoteTestSession(t, mainAdapter, true, nil)
	session.extractorAdapter = extractAdapter
	ctx := context.Background()

	result, err := session.Send(ctx, "I live in Lisbon, please remember that")
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if result == nil || result.Reply != "Noted." {
		t.Fatalf("Send() = %#v, want reply %q", result, "Noted.")
	}

	deadline := time.After(5 * time.Second)
	for len(extractAdapter.snapshot()) < 1 {
		select {
		case <-extractAdapter.callDone:
		case <-deadline:
			t.Fatal("background memory extractor did not run")
		}
	}
	session.Close(ctx)

	stored, err := noteRepo.ListByProfile(ctx, "p")
	if err != nil || len(stored) != 1 {
		t.Fatalf("ListByProfile() = %d notes, %v; want 1 (extractor merges into the tool-saved note, not a duplicate)", len(stored), err)
	}
	msgs, err := session.msgRepo.ListByConversation(ctx, "conv")
	if err != nil {
		t.Fatal(err)
	}
	var assistantMsgID string
	for _, m := range msgs {
		if m.Role == store.RoleAssistant {
			assistantMsgID = m.ID
			break
		}
	}
	if assistantMsgID == "" || !strings.Contains(stored[0].SourceMessageIDs, `"`+assistantMsgID+`"`) {
		t.Fatalf("extraction source IDs not merged into tool-saved note: %s", stored[0].SourceMessageIDs)
	}
}

func TestSendToolSaveFooterCountsSurviveExtraction(t *testing.T) {
	mainAdapter := &chatRecordingAdapter{responses: []*provider.CompletionResponse{
		{ToolCalls: []provider.ToolCall{{ID: "call_1", CallID: "call_1", Name: "save_note", Arguments: `{"content":"User lives in Porto","category":"fact","importance":6}`}}},
		{Content: "Noted.", Model: "chat"},
	}}
	extractAdapter := &chatRecordingAdapter{callDone: make(chan struct{}, 4), responses: []*provider.CompletionResponse{
		{Content: `{"has_new_info":false,"confidence":0,"notes":[]}`},
	}}
	var mu sync.Mutex
	var notesCalls [][2]int
	onNotes := func(saved, updated int) {
		mu.Lock()
		notesCalls = append(notesCalls, [2]int{saved, updated})
		mu.Unlock()
	}
	session, _ := newSaveNoteTestSession(t, mainAdapter, true, onNotes)
	session.extractorAdapter = extractAdapter
	ctx := context.Background()

	if _, err := session.Send(ctx, "I live in Porto, note that"); err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	deadline := time.After(5 * time.Second)
	for len(extractAdapter.snapshot()) < 1 {
		select {
		case <-extractAdapter.callDone:
		case <-deadline:
			t.Fatal("background memory extractor did not run")
		}
	}
	mu.Lock()
	last := notesCalls[len(notesCalls)-1]
	mu.Unlock()
	if last != [2]int{1, 0} {
		t.Fatalf("final onNotes call = %v, want (1,0): tool-save count was lost to the extractor's footer call", last)
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
