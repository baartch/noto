package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"noto/internal/chat"
	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
)

const testOrigin = "http://127.0.0.1:8080"

func TestAPIStateHistoryAndMemory(t *testing.T) {
	db := newWebTestDB(t)
	profile := &store.Profile{ID: "p1", Name: "Artist", Slug: "artist"}
	if err := store.NewConversationRepo(db).Create(context.Background(), &store.Conversation{ID: "c1", ProfileID: profile.ID, Status: store.ConversationArchived}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewMessageRepo(db).Create(context.Background(), &store.Message{ID: "m1", ConversationID: "c1", Role: store.RoleUser, Content: "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewMemoryNoteRepo(db).Create(context.Background(), &store.MemoryNote{ID: "n1", ProfileID: profile.ID, Category: store.CategoryFact, Content: "remember this", Importance: 7}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewMemorySummaryRepo(db).Upsert(context.Background(), &store.MemorySummary{
		ID: "w1", ProfileID: profile.ID, SummaryType: store.SummaryTypeWeekly, PeriodKey: "2026-W41",
		PeriodStart: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 10, 11, 23, 59, 59, 0, time.UTC),
		Content: "weekly summary", FreshnessState: store.SummaryFresh,
	}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(NewService(db, profile, &testChatSession{pending: 2}, "model-x", false), testOrigin)

	state := requestJSON(t, h, http.MethodGet, "/api/state", "")
	var stateBody struct {
		Profile struct {
			Name string `json:"name"`
		} `json:"profile"`
		Model         string `json:"model"`
		SetupRequired bool   `json:"setup_required"`
		PendingNotes  int    `json:"pending_notes"`
	}
	if err := json.Unmarshal(state, &stateBody); err != nil {
		t.Fatal(err)
	}
	if stateBody.Profile.Name != "Artist" || stateBody.Model != "model-x" || stateBody.SetupRequired || stateBody.PendingNotes != 2 {
		t.Fatalf("state = %+v", stateBody)
	}

	messages := requestJSON(t, h, http.MethodGet, "/api/messages?limit=1", "")
	var messagePage struct {
		Items []MessageDTO `json:"items"`
		Next  *string      `json:"next_cursor"`
	}
	if err := json.Unmarshal(messages, &messagePage); err != nil || len(messagePage.Items) != 1 || messagePage.Items[0].ID != "m1" {
		t.Fatalf("messages = %+v, err=%v", messagePage, err)
	}

	daily := requestJSON(t, h, http.MethodGet, "/api/memory?tab=daily&limit=10", "")
	weekly := requestJSON(t, h, http.MethodGet, "/api/memory?tab=weekly&limit=10", "")
	monthly := requestJSON(t, h, http.MethodGet, "/api/memory?tab=monthly&limit=10", "")
	var dailyPage, weeklyPage, monthlyPage MemoryPageDTO
	if err := json.Unmarshal(daily, &dailyPage); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(weekly, &weeklyPage); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(monthly, &monthlyPage); err != nil {
		t.Fatal(err)
	}
	if len(dailyPage.Items) != 1 || dailyPage.Items[0].Content != "remember this" {
		t.Fatalf("daily notes = %+v", dailyPage.Items)
	}
	if len(weeklyPage.Items) != 1 || weeklyPage.Items[0].Content != "weekly summary" {
		t.Fatalf("weekly summaries = %+v", weeklyPage.Items)
	}
	if monthlyPage.Items == nil || len(monthlyPage.Items) != 0 {
		t.Fatalf("missing monthly summaries = %+v, want empty array", monthlyPage.Items)
	}
}

func TestAPIMissingProviderIsReadOnlyAndValidationErrors(t *testing.T) {
	db := newWebTestDB(t)
	profile := &store.Profile{ID: "p1", Name: "Artist", Slug: "artist"}
	h := NewHandler(NewService(db, profile, nil, "", true), testOrigin)
	state := requestJSON(t, h, http.MethodGet, "/api/state", "")
	var stateBody struct {
		SetupRequired bool `json:"setup_required"`
	}
	if err := json.Unmarshal(state, &stateBody); err != nil || !stateBody.SetupRequired {
		t.Fatalf("missing-provider state = %+v, err=%v", stateBody, err)
	}

	badPaths := []string{
		"/api/messages?limit=0",
		"/api/messages?limit=101",
		"/api/messages?before=not-a-cursor",
		"/api/memory?tab=unknown",
		"/api/memory?tab=daily&offset=-1",
		"/api/memory?tab=weekly&limit=101",
	}
	for _, path := range badPaths {
		t.Run(path, func(t *testing.T) {
			recorder := request(h, http.MethodGet, path, "", "")
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("GET %s status = %d, want 400: %s", path, recorder.Code, recorder.Body.String())
			}
		})
	}
	if recorder := request(h, http.MethodGet, "/not-found", "", ""); recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want 404", recorder.Code)
	}
	if recorder := request(h, http.MethodPost, "/api/state", `{}`, testOrigin); recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unsupported method status = %d, want 405", recorder.Code)
	}
	if recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"r1","content":"hello"}`, testOrigin); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("send without provider status = %d, want 503", recorder.Code)
	}
}

func TestAPISecurityAndSendErrors(t *testing.T) {
	db := newWebTestDB(t)
	profile := &store.Profile{ID: "p1", Name: "Artist", Slug: "artist"}
	session := &testChatSession{send: func(context.Context, string) (*chat.SendResult, error) {
		return nil, errors.New("upstream rejected key secret-value")
	}}
	h := NewHandler(NewService(db, profile, session, "model", false), testOrigin)
	var logOutput strings.Builder
	previousLog := log.Writer()
	log.SetOutput(&logOutput)
	defer log.SetOutput(previousLog)

	if recorder := requestWithHost(h, http.MethodGet, "/api/state", "", "", "attacker.invalid"); recorder.Code != http.StatusMisdirectedRequest {
		t.Fatalf("foreign Host status = %d, want 421", recorder.Code)
	}
	if recorder := request(h, http.MethodGet, "/api/state", "", "http://evil.example"); recorder.Code != http.StatusForbidden {
		t.Fatalf("foreign Origin status = %d, want 403", recorder.Code)
	}
	state := request(h, http.MethodGet, "/api/state", "", "")
	if state.Header().Get("Cache-Control") != "no-store" || state.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("API headers lack no-store or unexpectedly enable CORS: %v", state.Header())
	}
	if policy := state.Header().Get("Content-Security-Policy"); !strings.Contains(policy, "default-src 'self'") || !strings.Contains(policy, "object-src 'none'") {
		t.Fatalf("CSP = %q", policy)
	}

	if recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"r1","content":"hello"}`, testOrigin); recorder.Code != http.StatusBadGateway {
		t.Fatalf("provider failure status = %d, want 502", recorder.Code)
	} else if strings.Contains(recorder.Body.String(), "secret-value") || strings.Contains(recorder.Body.String(), "upstream rejected") {
		t.Fatalf("provider details leaked in error: %s", recorder.Body.String())
	}
	if !strings.Contains(logOutput.String(), "category=chat_error") || strings.Contains(logOutput.String(), "secret-value") {
		t.Fatalf("server error log = %q, want sanitized failure category without provider secret", logOutput.String())
	}
	if recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"r2","content":"   "}`, testOrigin); recorder.Code != http.StatusBadRequest {
		t.Fatalf("blank message status = %d, want 400", recorder.Code)
	}
	if recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"r3","content":"hello","unexpected":1}`, testOrigin); recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown JSON field status = %d, want 400", recorder.Code)
	}
	oversizedJSON := fmt.Sprintf(`{"request_id":"large","content":"%s"}`, strings.Repeat("x", 64*1024))
	if recorder := request(h, http.MethodPost, "/api/messages", oversizedJSON, testOrigin); recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body status = %d, want 413", recorder.Code)
	}
	if recorder := requestNoJSON(h, http.MethodPost, "/api/messages", `{ "request_id":"r4", "content":"hello" }`, testOrigin); recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("non-JSON content type status = %d, want 415", recorder.Code)
	}
}

func TestProviderFailureStillPersistsUserMessageWithoutLeakingError(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	db := newWebTestDB(t)
	profile := &store.Profile{ID: "p1", Name: "Artist", Slug: ""}
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream rejected secret-value"}}`))
	}))
	defer providerServer.Close()
	conversationRepo := store.NewConversationRepo(db)
	messageRepo := store.NewMessageRepo(db)
	sessionLogger := &captureLogger{}
	session, err := chat.NewSession(
		context.Background(), profile.ID, profile.Slug, "system prompt", db,
		conversationRepo, messageRepo, store.NewMemoryNoteRepo(db),
		provider.NewOpenAICompatible(provider.Config{Endpoint: providerServer.URL, Model: "model", APIKey: "secret-value"}),
		nil, sessionLogger, nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close(context.Background())
	h := NewHandler(NewService(db, profile, session, "model", false), testOrigin)
	recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"persisted-failure","content":"keep this turn"}`, testOrigin)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("provider failure status = %d, want 502: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-value") || strings.Contains(recorder.Body.String(), "upstream rejected") {
		t.Fatalf("provider details leaked: %s", recorder.Body.String())
	}
	if logged := sessionLogger.String(); strings.Contains(logged, "secret-value") {
		t.Fatalf("provider detail leaked to session log: %s", logged)
	}
	conversation, err := conversationRepo.GetMostRecentByProfile(context.Background(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := messageRepo.ListByConversation(context.Background(), conversation.ID)
	if err != nil || len(messages) != 1 || messages[0].Role != store.RoleUser || messages[0].Content != "keep this turn" {
		t.Fatalf("persisted messages = %+v, err=%v", messages, err)
	}
}

type captureLogger struct {
	mu     sync.Mutex
	output strings.Builder
}

func (*captureLogger) Emit(observe.Event)   {}
func (*captureLogger) Infof(string, ...any) {}
func (l *captureLogger) Errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(&l.output, format+"\n", args...)
}
func (l *captureLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.output.String()
}

func TestSendSingleFlightRequestIDReuseAndClientDisconnect(t *testing.T) {
	db := newWebTestDB(t)
	profile := &store.Profile{ID: "p1", Name: "Artist", Slug: "artist"}
	session := newBlockingChatSession()
	h := NewHandler(NewService(db, profile, session, "model", false), testOrigin)

	firstRecorder := httptest.NewRecorder()
	firstRequest := newAPIRequest(context.Background(), http.MethodPost, "/api/messages", `{"request_id":"r1","content":"hello"}`, testOrigin)
	firstDone := make(chan struct{})
	go func() {
		h.ServeHTTP(firstRecorder, firstRequest)
		close(firstDone)
	}()
	select {
	case <-session.started:
	case <-time.After(2 * time.Second):
		t.Fatal("first send did not start")
	}

	if recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"r2","content":"different"}`, testOrigin); recorder.Code != http.StatusConflict {
		t.Fatalf("competing send status = %d, want 409", recorder.Code)
	}
	if recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"r1","content":"different"}`, testOrigin); recorder.Code != http.StatusConflict {
		t.Fatalf("request ID with different content status = %d, want 409", recorder.Code)
	}

	duplicateRecorder := httptest.NewRecorder()
	duplicateRequest := newAPIRequest(context.Background(), http.MethodPost, "/api/messages", `{"request_id":"r1","content":"hello"}`, testOrigin)
	duplicateDone := make(chan struct{})
	go func() {
		h.ServeHTTP(duplicateRecorder, duplicateRequest)
		close(duplicateDone)
	}()

	close(session.release)
	for _, done := range []<-chan struct{}{firstDone, duplicateDone} {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("send handler did not finish")
		}
	}
	if firstRecorder.Code != http.StatusOK || duplicateRecorder.Code != http.StatusOK || session.callCount() != 1 {
		t.Fatalf("statuses/calls = %d/%d/%d, want 200/200/1", firstRecorder.Code, duplicateRecorder.Code, session.callCount())
	}

	ctx, cancel := context.WithCancel(context.Background())
	disconnected := httptest.NewRecorder()
	disconnectedDone := make(chan struct{})
	go func() {
		h.ServeHTTP(disconnected, newAPIRequest(ctx, http.MethodPost, "/api/messages", `{"request_id":"r3","content":"still send"}`, testOrigin))
		close(disconnectedDone)
	}()
	select {
	case <-session.started:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnected send did not start")
	}
	cancel()
	select {
	case <-disconnectedDone:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnected request handler did not return")
	}
	state := requestJSON(t, h, http.MethodGet, "/api/state", "")
	var status struct {
		Sending bool `json:"sending"`
	}
	if err := json.Unmarshal(state, &status); err != nil || !status.Sending {
		t.Fatalf("state after client disconnect = %+v, err=%v; send should continue", status, err)
	}
	close(session.releaseSecond)
	select {
	case <-session.secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("accepted send did not finish after client disconnect")
	}
}

type testChatSession struct {
	pending int
	send    func(context.Context, string) (*chat.SendResult, error)
}

func (s *testChatSession) PendingNotes() int { return s.pending }
func (s *testChatSession) Send(ctx context.Context, content string) (*chat.SendResult, error) {
	if s.send != nil {
		return s.send(ctx, content)
	}
	return &chat.SendResult{Reply: "reply"}, nil
}

type blockingSession struct {
	started       chan struct{}
	release       chan struct{}
	releaseSecond chan struct{}
	secondDone    chan struct{}
	mu            sync.Mutex
	calls         int
}

func newBlockingChatSession() *blockingSession {
	return &blockingSession{started: make(chan struct{}, 1), release: make(chan struct{}), releaseSecond: make(chan struct{}), secondDone: make(chan struct{}), calls: 0}
}

func (s *blockingSession) PendingNotes() int { return 0 }
func (s *blockingSession) Send(ctx context.Context, content string) (*chat.SendResult, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	s.started <- struct{}{}
	if call == 1 {
		<-s.release
	} else {
		<-s.releaseSecond
		defer close(s.secondDone)
	}
	return &chat.SendResult{Reply: "reply:" + content}, nil
}
func (s *blockingSession) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func newWebTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.OpenForTesting(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func requestJSON(t *testing.T, h http.Handler, method, path, body string) []byte {
	t.Helper()
	recorder := request(h, method, path, body, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d: %s", method, path, recorder.Code, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}

func request(h http.Handler, method, path, body, origin string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, newAPIRequest(context.Background(), method, path, body, origin))
	return recorder
}

func requestWithHost(h http.Handler, method, path, body, origin, host string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := newAPIRequest(context.Background(), method, path, body, origin)
	req.Host = host
	h.ServeHTTP(recorder, req)
	return recorder
}

func requestNoJSON(h http.Handler, method, path, body, origin string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := newAPIRequest(context.Background(), method, path, body, origin)
	req.Header.Set("Content-Type", "text/plain")
	h.ServeHTTP(recorder, req)
	return recorder
}

func newAPIRequest(ctx context.Context, method, path, body, origin string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:8080"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
