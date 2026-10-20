package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"noto/internal/app"
	"noto/internal/config"
	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
)

const webExtractorReply = `{"has_new_info":true,"confidence":0.9,"notes":[{"action":"add","target_id":"","category":"fact","content":"User lives in Helsinki.","importance":7}]}`

func TestWebFlowSharesProfileHistoryNotesAndSummaries(t *testing.T) {
	profile, dbPath := seedWebIntegrationProfile(t)
	provider := newWebIntegrationProvider(t, false)
	configureWebEnvironment(t, provider.server.URL)
	process := startWebIntegrationProcess(t)
	defer func() {
		provider.releaseExtraction()
		process.cancel()
		process.wait(t)
	}()

	page := webGet(t, process.url, "/")
	if !strings.Contains(string(page), "Noto — Workspace") || !strings.Contains(string(page), "theme.css") {
		t.Fatalf("web page did not serve embedded workspace: %q", page[:min(len(page), 200)])
	}
	initialHistory := webGet(t, process.url, "/api/messages?limit=50")
	if !strings.Contains(string(initialHistory), "terminal history message") || strings.Contains(string(initialHistory), "other profile message") {
		t.Fatalf("profile history = %s", initialHistory)
	}
	assertSummaryContent(t, process.url, "weekly", "existing weekly fixture")
	assertSummaryContent(t, process.url, "monthly", "existing monthly fixture")

	response := webPost(t, process.url, "web-flow-1", "Remember I live in Helsinki.")
	var result struct {
		Reply struct {
			Content string `json:"content"`
		} `json:"reply"`
	}
	if err := json.Unmarshal(response, &result); err != nil || result.Reply.Content != "I will remember that you live in Helsinki." {
		t.Fatalf("chat response = %+v, err=%v", result, err)
	}
	firstRequest := awaitProviderRequest(t, provider, func(request providerChatRequest) bool {
		return hasProviderMessage(request, "Remember I live in Helsinki.")
	})
	if !hasProviderMessage(firstRequest, "terminal history message") {
		t.Fatalf("new web session omitted persisted terminal history: %+v", firstRequest.Messages)
	}
	waitForWebNote(t, process.url, "User lives in Helsinki.")
	finalHistory := webGet(t, process.url, "/api/messages?limit=50")
	for _, content := range []string{"terminal history message", "Remember I live in Helsinki.", "I will remember that you live in Helsinki."} {
		if !strings.Contains(string(finalHistory), content) {
			t.Errorf("profile history omitted %q: %s", content, finalHistory)
		}
	}
	daily := webGet(t, process.url, "/api/memory?tab=daily&limit=50")
	if !strings.Contains(string(daily), "User lives in Helsinki.") || strings.Contains(string(daily), "other profile memory") {
		t.Fatalf("daily memory profile isolation/content = %s", daily)
	}

	db, err := store.OpenProfile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := store.NewMemoryNoteRepo(db).ListByProfile(context.Background(), profile.ID)
	_ = db.Close()
	if err != nil || len(notes) != 1 || notes[0].Content != "User lives in Helsinki." {
		t.Fatalf("persisted profile notes = %+v, err=%v", notes, err)
	}
}

func TestWebLaunchLifecycleMatchesChat(t *testing.T) {
	profile, dbPath := seedWebIntegrationProfile(t)
	provider := newWebIntegrationProvider(t, true)
	configureWebEnvironment(t, provider.server.URL)
	first := startWebIntegrationProcess(t)
	defer func() {
		provider.releaseExtraction()
		first.cancel()
		select {
		case <-first.done:
		case <-time.After(6 * time.Second):
			t.Error("first web command did not stop during cleanup")
		}
	}()
	firstCount := countWebConversations(t, dbPath, profile.ID)
	if firstCount != 2 {
		t.Fatalf("conversation count at first web startup = %d, want seeded + one web session", firstCount)
	}
	for range 2 {
		_ = webGet(t, first.url, "/")
		_ = webGet(t, first.url, "/api/messages?limit=50")
	}
	if got := countWebConversations(t, dbPath, profile.ID); got != firstCount {
		t.Fatalf("browser reload created conversations: got %d, want %d", got, firstCount)
	}
	_ = webPost(t, first.url, "lifecycle-1", "Remember I live in Helsinki.")
	_ = awaitProviderRequest(t, provider, func(request providerChatRequest) bool {
		return hasProviderMessage(request, "Remember I live in Helsinki.")
	})
	select {
	case <-provider.extractionStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("async note extraction did not start")
	}

	first.cancel()
	select {
	case <-first.done:
		t.Fatalf("web shutdown returned while extraction was blocked: %v", first.err)
	case <-time.After(50 * time.Millisecond):
	}
	provider.releaseExtraction()
	first.wait(t)
	waitForStoredNote(t, dbPath, profile.ID, "User lives in Helsinki.")
	if got := countWebConversations(t, dbPath, profile.ID); got != firstCount {
		t.Fatalf("shutdown changed conversation count: got %d, want %d", got, firstCount)
	}

	second := startWebIntegrationProcess(t)
	defer func() {
		provider.releaseExtraction()
		second.cancel()
		second.wait(t)
	}()
	secondCount := countWebConversations(t, dbPath, profile.ID)
	if secondCount != firstCount+1 {
		t.Fatalf("conversation count at next process startup = %d, want %d", secondCount, firstCount+1)
	}
	_ = webPost(t, second.url, "lifecycle-2", "What did I tell you earlier?")
	followup := awaitProviderRequest(t, provider, func(request providerChatRequest) bool {
		return hasProviderMessage(request, "What did I tell you earlier?")
	})
	if !hasProviderMessage(followup, "Remember I live in Helsinki.") || !hasProviderMessage(followup, "I will remember that you live in Helsinki.") {
		t.Fatalf("new process did not carry recent conversation context: %+v", followup.Messages)
	}
}

func seedWebIntegrationProfile(t *testing.T) (*store.Profile, string) {
	t.Helper()
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	ctx := context.Background()
	profiles := profile.NewService(nil)
	p, err := profiles.Create(ctx, "Web Integration")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profiles.Select(ctx, p.Name); err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureAppDirs(p.Slug); err != nil {
		t.Fatal(err)
	}
	dbPath, err := config.ProfileDBPath(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenProfile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	conversationRepo := store.NewConversationRepo(db)
	messageRepo := store.NewMessageRepo(db)
	if err := conversationRepo.Create(ctx, &store.Conversation{ID: "terminal-history", ProfileID: p.ID, Status: store.ConversationArchived}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE conversations SET started_at = ? WHERE id = ?`, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "terminal-history"); err != nil {
		t.Fatal(err)
	}
	if err := messageRepo.Create(ctx, &store.Message{ID: "terminal-message", ConversationID: "terminal-history", Role: store.RoleUser, Content: "terminal history message"}); err != nil {
		t.Fatal(err)
	}
	if err := conversationRepo.Create(ctx, &store.Conversation{ID: "other-history", ProfileID: "other-profile", Status: store.ConversationArchived}); err != nil {
		t.Fatal(err)
	}
	if err := messageRepo.Create(ctx, &store.Message{ID: "other-message", ConversationID: "other-history", Role: store.RoleUser, Content: "other profile message"}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewMemoryNoteRepo(db).Create(ctx, &store.MemoryNote{ID: "other-note", ProfileID: "other-profile", Category: store.CategoryFact, Content: "other profile memory", Importance: 8}); err != nil {
		t.Fatal(err)
	}
	stateVersion := memory.SummaryStateVersion([]string{})
	for _, summary := range []*store.MemorySummary{
		{
			ID: "existing-weekly", ProfileID: p.ID, SummaryType: store.SummaryTypeWeekly, PeriodKey: "2026-W40",
			PeriodStart: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 10, 4, 23, 59, 59, 0, time.UTC),
			Content: "existing weekly fixture", SourceStateVersion: stateVersion, FreshnessState: store.SummaryFresh,
		},
		{
			ID: "existing-monthly", ProfileID: p.ID, SummaryType: store.SummaryTypeMonthly, PeriodKey: "2026-09",
			PeriodStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 9, 30, 23, 59, 59, 0, time.UTC),
			Content: "existing monthly fixture", SourceStateVersion: stateVersion, FreshnessState: store.SummaryFresh,
		},
	} {
		if err := store.NewMemorySummaryRepo(db).Upsert(ctx, summary); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return p, dbPath
}

type providerChatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type webIntegrationProvider struct {
	server            *httptest.Server
	requests          chan providerChatRequest
	extractionStarted chan struct{}
	release           chan struct{}
	releaseOnce       sync.Once
	blockExtraction   bool
	completions       atomic.Int32
}

func newWebIntegrationProvider(t *testing.T, blockExtraction bool) *webIntegrationProvider {
	t.Helper()
	stub := &webIntegrationProvider{
		requests: make(chan providerChatRequest, 8), extractionStarted: make(chan struct{}, 1),
		release: make(chan struct{}), blockExtraction: blockExtraction,
	}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"chat-model","context_length":8192,"supported_parameters":[]}]}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var request providerChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		stub.requests <- request
		call := stub.completions.Add(1)
		content := "I will remember that you live in Helsinki."
		switch {
		case call == 2:
			stub.extractionStarted <- struct{}{}
			if stub.blockExtraction {
				select {
				case <-stub.release:
				case <-r.Context().Done():
					return
				}
			}
			content = webExtractorReply
		case call == 3:
			content = "You told me that you live in Helsinki."
		case call > 3:
			content = webExtractorReply
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"model":"chat-model","choices":[{"message":{"role":"assistant","content":%q}}]}`, content)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (p *webIntegrationProvider) releaseExtraction() {
	p.releaseOnce.Do(func() { close(p.release) })
}

func configureWebEnvironment(t *testing.T, endpoint string) {
	t.Helper()
	t.Setenv("NOTO_API_KEY", "integration-secret")
	t.Setenv("NOTO_ENDPOINT", endpoint)
	t.Setenv("NOTO_MODEL", "chat-model")
}

type webCommandProcess struct {
	url    string
	cancel context.CancelFunc
	done   chan struct{}
	err    error
	output *webCommandOutput
}

func startWebIntegrationProcess(t *testing.T) *webCommandProcess {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	output := &webCommandOutput{changed: make(chan struct{}, 1)}
	command := app.RootCmd()
	command.SetContext(ctx)
	command.SetArgs([]string{"web", "--port", "0"})
	command.SetOut(output)
	command.SetErr(output)
	process := &webCommandProcess{cancel: cancel, done: make(chan struct{}), output: output}
	go func() {
		process.err = command.Execute()
		close(process.done)
	}()
	pattern := regexp.MustCompile(`Server running on port ([0-9]+): (http://127\.0\.0\.1:[0-9]+)`)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		text := output.String()
		if match := pattern.FindStringSubmatch(text); len(match) == 3 {
			process.url = match[2]
			return process
		}
		select {
		case <-output.changed:
		case <-process.done:
			t.Fatalf("web command exited before startup: err=%v output=%q", process.err, output.String())
		case <-deadline.C:
			cancel()
			t.Fatalf("web command did not report startup: %q", output.String())
		}
	}
}

func (p *webCommandProcess) wait(t *testing.T) {
	t.Helper()
	select {
	case <-p.done:
		if p.err != nil {
			t.Fatalf("web command returned error: %v; output=%q", p.err, p.output.String())
		}
	case <-time.After(6 * time.Second):
		t.Fatal("web command did not stop")
	}
}

type webCommandOutput struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	changed chan struct{}
}

func (w *webCommandOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	n, err := w.buffer.Write(data)
	w.mu.Unlock()
	select {
	case w.changed <- struct{}{}:
	default:
	}
	return n, err
}

func (w *webCommandOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func webGet(t *testing.T, baseURL, path string) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("create GET %s: %v", path, err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer response.Body.Close()
	var body bytes.Buffer
	if _, err := body.ReadFrom(response.Body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d: %s", path, response.StatusCode, body.String())
	}
	return body.Bytes()
}

func webPost(t *testing.T, baseURL, requestID, content string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"request_id": requestID, "content": content})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/messages", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", baseURL)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("POST message: %v", err)
	}
	defer response.Body.Close()
	var body bytes.Buffer
	if _, err := body.ReadFrom(response.Body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST message status = %d: %s", response.StatusCode, body.String())
	}
	return body.Bytes()
}

func assertSummaryContent(t *testing.T, baseURL, tab, expected string) {
	t.Helper()
	body := webGet(t, baseURL, "/api/memory?tab="+tab+"&limit=50")
	if !strings.Contains(string(body), expected) {
		t.Fatalf("%s summary not returned: %s", tab, body)
	}
}

func waitForWebNote(t *testing.T, baseURL, expected string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		body := webGet(t, baseURL, "/api/memory?tab=daily&limit=50")
		if strings.Contains(string(body), expected) {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("timed out waiting for note %q; last response=%s", expected, body)
		}
	}
}

func waitForStoredNote(t *testing.T, dbPath, profileID, expected string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		db, err := store.OpenProfile(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		notes, listErr := store.NewMemoryNoteRepo(db).ListByProfile(context.Background(), profileID)
		_ = db.Close()
		if listErr != nil {
			t.Fatal(listErr)
		}
		for _, note := range notes {
			if note.Content == expected {
				return
			}
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("timed out waiting for persisted note %q", expected)
		}
	}
}

func countWebConversations(t *testing.T, dbPath, profileID string) int {
	t.Helper()
	db, err := store.OpenProfile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	conversations, err := store.NewConversationRepo(db).ListByProfile(context.Background(), profileID)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}
	return len(conversations)
}

func awaitProviderRequest(t *testing.T, provider *webIntegrationProvider, match func(providerChatRequest) bool) providerChatRequest {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case request := <-provider.requests:
			if match(request) {
				return request
			}
		case <-timer.C:
			t.Fatal("timed out waiting for expected provider request")
		}
	}
}

func hasProviderMessage(request providerChatRequest, content string) bool {
	for _, message := range request.Messages {
		if strings.Contains(message.Content, content) {
			return true
		}
	}
	return false
}
