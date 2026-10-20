package chat

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/observe"

	"noto/internal/provider"
	"noto/internal/reference"
	"noto/internal/store"
)

type chatRecordingAdapter struct {
	mu        sync.Mutex
	requests  []provider.CompletionRequest
	responses []*provider.CompletionResponse
	callDone  chan struct{}
}

func (a *chatRecordingAdapter) Complete(_ context.Context, request provider.CompletionRequest) (*provider.CompletionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, request)
	if a.callDone != nil {
		select {
		case a.callDone <- struct{}{}:
		default:
		}
	}
	idx := len(a.requests) - 1
	if idx >= len(a.responses) {
		idx = len(a.responses) - 1
	}
	return a.responses[idx], nil
}
func (*chatRecordingAdapter) Embed(context.Context, provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return nil, nil
}
func (*chatRecordingAdapter) ProviderType() string { return "test" }
func (a *chatRecordingAdapter) snapshot() []provider.CompletionRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]provider.CompletionRequest(nil), a.requests...)
}

func TestBuildReferenceMessagesPreservesPrefixAndOrdersAttachments(t *testing.T) {
	history := []*store.Message{
		{ID: "u1", Role: store.RoleUser, Content: "I am an artist"},
		{ID: "a1", Role: store.RoleAssistant, Content: "Tell me more"},
		{ID: "u2", Role: store.RoleUser, Content: "Write an artist bio"},
	}
	attachments := []reference.Attachment{
		{ID: "b", RelativePath: "z/voice.txt", Fingerprint: "fb", Text: "second document"},
		{ID: "a", RelativePath: "identity.md", Fingerprint: "fa", Text: "first document"},
	}
	current := "[Current datetime: now]\n\nWrite an artist bio"
	withDocs := buildReferenceMessages("system prompt", `[{"id":"a"}]`, history, attachments, current)
	withoutDocs := buildReferenceMessages("system prompt", `[{"id":"a"}]`, history, nil, current)
	firstAttachment := -1
	for i, message := range withDocs {
		if strings.Contains(message.Content, "<noto-reference>") {
			firstAttachment = i
			break
		}
	}
	if firstAttachment < 0 {
		t.Fatal("reference attachments were not added")
	}
	if !reflect.DeepEqual(withDocs[:firstAttachment], withoutDocs[:firstAttachment]) {
		t.Fatal("adding documents changed the leading request prefix")
	}
	if got := withDocs[len(withDocs)-1]; got.Role != "user" || got.Content != current {
		t.Fatalf("current user turn is not last and exact: %#v", got)
	}
	if strings.Count(withDocs[len(withDocs)-1].Content, "Write an artist bio") != 1 {
		t.Fatal("current user turn was duplicated")
	}
	if withDocs[firstAttachment].Role != "user" || !strings.Contains(withDocs[firstAttachment].Content, "first document") || strings.Contains(withDocs[firstAttachment].Content, "second document") {
		t.Fatalf("attachments not sorted/delimited: %#v", withDocs[firstAttachment])
	}
	if withDocs[1].Role != "user" || !strings.Contains(withDocs[1].Content, "untrusted") {
		t.Fatalf("catalog must be a distinct untrusted user reference: %#v", withDocs[1])
	}
	for _, message := range withDocs {
		if message.Role == "system" && strings.Contains(message.Content, "first document") {
			t.Fatal("raw attachment text entered system instructions")
		}
	}
}

func TestBuildReferenceMessagesOmitsEmptyCatalogAndDocuments(t *testing.T) {
	messages := buildReferenceMessages("system", "", []*store.Message{{Role: store.RoleUser, Content: "current"}}, nil, "dated current")
	want := []provider.Message{{Role: "system", Content: "system"}, {Role: "user", Content: "dated current"}}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("messages = %#v, want %#v", messages, want)
	}
}

func TestSessionSendKeepsReferenceTextTemporaryAndAtRequestTail(t *testing.T) {
	root := t.TempDir()
	body := "PRIVATE SOURCE ONLY"
	if err := os.WriteFile(filepath.Join(root, "identity.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	convRepo := store.NewConversationRepo(db)
	if err := convRepo.Create(ctx, &store.Conversation{ID: "conv", ProfileID: "p", Status: store.ConversationActive}); err != nil {
		t.Fatal(err)
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := reference.Read(ctx, rootHandle, "identity.md")
	_ = rootHandle.Close()
	if err != nil {
		t.Fatal(err)
	}
	fileRepo := store.NewReferenceFileRepo(db)
	if err := fileRepo.Upsert(ctx, store.ReferenceFile{
		ID: "rf-artist", ProfileID: "p", RelativePath: "identity.md", MediaType: doc.MediaType,
		Fingerprint: doc.Fingerprint, SummaryFingerprint: doc.Fingerprint,
		Summary: "Artist identity and public voice.", Status: store.ReferenceFileReady,
	}); err != nil {
		t.Fatal(err)
	}
	selectorAdapter := &chatRecordingAdapter{callDone: make(chan struct{}, 4), responses: []*provider.CompletionResponse{
		{Content: `{"topic":"artist identity","topic_changed":false,"file_ids":["rf-artist"]}`, Usage: provider.Usage{PromptTokens: 2, HasUsage: true}},
		{Content: `{"has_new_info":false,"confidence":0,"notes":[]}`},
	}}
	mainAdapter := &chatRecordingAdapter{responses: []*provider.CompletionResponse{{Content: "Here is your bio.", Model: "chat", PromptTokens: 10, CompletionTokens: 3}}}
	msgRepo := store.NewMessageRepo(db)
	session := &Session{
		profileID: "p", conversationID: "conv", systemPrompt: "system instructions",
		convRepo: convRepo, msgRepo: msgRepo, noteRepo: store.NewMemoryNoteRepo(db),
		adapter: mainAdapter, extractorAdapter: selectorAdapter, extractor: memory.NewExtractor(store.NewMemoryNoteRepo(db), selectorAdapter, store.NewContextCacheRepo(db)),
		cacheRepo: store.NewContextCacheRepo(db), db: db, logger: observe.NewJSONLogger(io.Discard),
		history:    []*store.Message{{ID: "old-user", Role: store.RoleUser, Content: "I am an artist"}, {ID: "old-assistant", Role: store.RoleAssistant, Content: "What is your style?"}},
		backupStop: make(chan struct{}), pendingDone: make(chan struct{}),
		referenceContextMax: 8192, modelContextMax: 8192,
	}
	session.references, err = reference.NewService(reference.Options{
		Root: root, ProfileID: "p", Repo: fileRepo,
		Model:   func() reference.Model { return reference.Model{Adapter: selectorAdapter, ContextMax: 8192} },
		OnUsage: session.recordAuxUsage,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.Send(ctx, "Write an artist bio")
	if err != nil || result.Reply != "Here is your bio." {
		t.Fatalf("Send() = %#v, %v", result, err)
	}
	deadline := time.After(2 * time.Second)
	for len(selectorAdapter.snapshot()) < 2 {
		select {
		case <-selectorAdapter.callDone:
		case <-deadline:
			t.Fatal("background memory extractor did not complete")
		}
	}
	selectorRequests := selectorAdapter.snapshot()
	for _, message := range selectorRequests[0].Messages {
		if strings.Contains(message.Content, body) {
			t.Fatal("reference selector received raw document content")
		}
	}
	for _, message := range selectorRequests[1].Messages {
		if strings.Contains(message.Content, body) || strings.Contains(message.Content, "Here is your bio.") {
			t.Fatal("reference-influenced assistant text reached memory extraction")
		}
	}
	requests := mainAdapter.snapshot()
	if len(requests) != 1 {
		t.Fatalf("main provider call count = %d", len(requests))
	}
	messages := requests[0].Messages
	if messages[0].Role != "system" || messages[0].Content != "system instructions" {
		t.Fatalf("system prefix changed: %#v", messages[0])
	}
	if messages[1].Role != "user" || !strings.Contains(messages[1].Content, "untrusted metadata") || !strings.Contains(messages[1].Content, "Artist identity") {
		t.Fatalf("file summary missing from catalog prefix: %#v", messages[1])
	}
	attachmentAt := -1
	for i, message := range messages {
		if strings.Contains(message.Content, "<noto-reference>") {
			attachmentAt = i
			break
		}
	}
	if attachmentAt < 0 || !strings.Contains(messages[attachmentAt].Content, body) || messages[len(messages)-1].Role != "user" || !strings.Contains(messages[len(messages)-1].Content, "Write an artist bio") {
		t.Fatalf("attachment/current message ordering incorrect: %#v", messages)
	}
	for _, message := range messages[:attachmentAt] {
		if strings.Contains(message.Content, body) {
			t.Fatal("reference text was not appended after stable prefix")
		}
	}
	stored, err := msgRepo.ListByConversation(ctx, "conv")
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range stored {
		if strings.Contains(message.Content, body) {
			t.Fatal("raw document text persisted as chat history")
		}
	}
	if len(stored) != 2 || !stored[1].ReferenceInfluenced {
		t.Fatalf("assistant provenance was not persisted: %#v", stored)
	}
	if session.Stats().TokensIn < 12 {
		t.Fatalf("selector usage was not accumulated: %#v", session.Stats())
	}
	session.Close(ctx)
}
