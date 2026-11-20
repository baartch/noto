package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"noto/internal/chat"
	"noto/internal/store"
)

func TestSettingsReadEndpoints(t *testing.T) {
	backend := &testSettingsBackend{}
	service := NewService(nil, &store.Profile{Name: "Artist", Slug: "artist"}, nil, "model-x", false)
	h := NewHandler(service, testOrigin, backend)

	profilesResponse := requestJSON(t, h, http.MethodGet, "/api/settings", "")
	var settings SettingsDTO
	if err := json.Unmarshal(profilesResponse, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.ActiveProfileSlug != "artist" || len(settings.Profiles) != 1 || settings.Profiles[0] != (SettingsProfileDTO{Slug: "artist", Name: "Artist"}) {
		t.Fatalf("settings = %+v", settings)
	}

	modelsResponse := requestJSON(t, h, http.MethodGet, "/api/settings/models?profile=artist", "")
	var models ProfileModelsDTO
	if err := json.Unmarshal(modelsResponse, &models); err != nil {
		t.Fatal(err)
	}
	if models.ProfileSlug != "artist" || models.SavedModel != "saved-model" || len(models.Models) != 2 || models.Models[0] != "model-a" || models.Models[1] != "model-b" {
		t.Fatalf("models = %+v", models)
	}
	if recorder := request(h, http.MethodPut, "/api/settings", `{}`, ""); recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /api/settings status = %d, want 405", recorder.Code)
	}
}

func TestSaveSettingsPassesOptionalModelSelections(t *testing.T) {
	backend := &testSettingsBackend{}
	h := NewHandler(NewService(nil, &store.Profile{ID: "artist", Name: "Artist", Slug: "artist"}, nil, "model-a", false), testOrigin, backend)
	body := `{"profile_slug":"artist","model":"model-b","embeddings_model":"embed-b","extractor_model":"","revision":1}`
	recorder := request(h, http.MethodPost, "/api/settings", body, testOrigin)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if backend.selection == nil || backend.selection.Model != "model-b" || backend.selection.EmbeddingsModel == nil || *backend.selection.EmbeddingsModel != "embed-b" || backend.selection.ExtractorModel == nil || *backend.selection.ExtractorModel != "" {
		t.Fatalf("model selection passed to backend = %#v", backend.selection)
	}
}

func TestSettingsModelErrorsAndRequestSecurity(t *testing.T) {
	backend := &testSettingsBackend{}
	h := NewHandler(NewService(nil, &store.Profile{Name: "Artist", Slug: "artist"}, nil, "model-x", false), testOrigin, backend)

	if recorder := request(h, http.MethodGet, "/api/settings/models", "", ""); recorder.Code != http.StatusBadRequest {
		t.Errorf("missing profile status = %d, want 400", recorder.Code)
	}
	backend.modelsErr = ErrSettingsProfileNotFound
	if recorder := request(h, http.MethodGet, "/api/settings/models?profile=missing", "", ""); recorder.Code != http.StatusNotFound {
		t.Errorf("unknown profile status = %d, want 404", recorder.Code)
	}
	backend.modelsErr = fmt.Errorf("%w: provider returned secret credential", ErrSettingsModelListUnavailable)
	if recorder := request(h, http.MethodGet, "/api/settings/models?profile=artist", "", ""); recorder.Code != http.StatusBadGateway {
		t.Errorf("provider failure status = %d, want 502", recorder.Code)
	} else if strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "credential") {
		t.Errorf("provider details leaked: %s", recorder.Body.String())
	}
	if recorder := requestWithHost(h, http.MethodGet, "/api/settings", "", "", "attacker.invalid"); recorder.Code != http.StatusMisdirectedRequest {
		t.Errorf("foreign host status = %d, want 421", recorder.Code)
	}
	if recorder := request(h, http.MethodGet, "/api/settings", "", "http://evil.example"); recorder.Code != http.StatusForbidden {
		t.Errorf("foreign origin status = %d, want 403", recorder.Code)
	}
}

func TestSettingsUnavailableWithoutBackend(t *testing.T) {
	h := NewHandler(NewService(nil, nil, nil, "", true), testOrigin)
	for _, path := range []string{"/api/settings", "/api/settings/models?profile=artist"} {
		if recorder := request(h, http.MethodGet, path, "", ""); recorder.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s status = %d, want 503", path, recorder.Code)
		}
	}
}

func TestSaveSettingsConflictsWithActiveSend(t *testing.T) {
	db := newWebTestDB(t)
	profile := &store.Profile{ID: "p1", Name: "Artist", Slug: "artist"}
	session := newBlockingChatSession()
	service := NewService(db, profile, session, "model-a", false)
	h := NewHandler(service, testOrigin, &testSettingsBackend{})

	sendDone := make(chan struct{})
	go func() {
		_, _ = service.Send(context.Background(), "active-settings-send", "hello")
		close(sendDone)
	}()
	select {
	case <-session.started:
	case <-time.After(2 * time.Second):
		t.Fatal("send did not start")
	}
	released := false
	defer func() {
		if !released {
			close(session.release)
		}
	}()

	body := `{"profile_slug":"artist","model":"model-b","revision":1}`
	if recorder := request(h, http.MethodPost, "/api/settings", body, testOrigin); recorder.Code != http.StatusConflict {
		t.Fatalf("save during send status = %d, want 409: %s", recorder.Code, recorder.Body.String())
	}
	close(session.release)
	released = true
	select {
	case <-sendDone:
	case <-time.After(2 * time.Second):
		t.Fatal("send did not finish")
	}
}

func TestSaveSettingsUpdatesRuntimeAndRejectsStaleRevision(t *testing.T) {
	service := NewService(nil, &store.Profile{ID: "a", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false)
	h := NewHandler(service, testOrigin, &testSettingsBackend{})
	body := `{"profile_slug":"beta","model":"model-b","revision":1}`
	recorder := request(h, http.MethodPost, "/api/settings", body, testOrigin)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	var state StateDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.Profile.Slug != "beta" || state.Model != "model-b" || state.Revision != 2 {
		t.Fatalf("saved state = %+v", state)
	}
	if recorder := request(h, http.MethodPost, "/api/settings", body, testOrigin); recorder.Code != http.StatusConflict {
		t.Fatalf("stale save status = %d, want 409: %s", recorder.Code, recorder.Body.String())
	}
}

func TestMessageDeduplicationIsScopedToProfile(t *testing.T) {
	aCalls, bCalls := 0, 0
	alpha := &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}
	betaSession := &testChatSession{send: func(context.Context, string) (*chat.SendResult, error) {
		bCalls++
		return &chat.SendResult{Reply: "beta reply"}, nil
	}}
	alphaSession := &testChatSession{send: func(context.Context, string) (*chat.SendResult, error) {
		aCalls++
		return &chat.SendResult{Reply: "alpha reply"}, nil
	}}
	service := NewService(nil, alpha, alphaSession, "model-a", false)
	first, err := service.SendForProfile(context.Background(), "same-id", "same content", "alpha")
	if err != nil || first.Reply.Content != "alpha reply" {
		t.Fatalf("alpha send = %+v, err=%v", first, err)
	}
	backend := &testSettingsBackend{nextSession: betaSession}
	if _, err := service.SaveSettings(context.Background(), backend, "beta", "model-b", 1); err != nil {
		t.Fatal(err)
	}
	second, err := service.SendForProfile(context.Background(), "same-id", "same content", "beta")
	if err != nil || second.Reply.Content != "beta reply" || aCalls != 1 || bCalls != 1 {
		t.Fatalf("beta send after profile switch = %+v, err=%v; calls A/B=%d/%d", second, err, aCalls, bCalls)
	}
}

func TestSettingsRecoveryRequiredHasDedicatedSafeResponse(t *testing.T) {
	backend := &testSettingsBackend{saveErr: ErrSettingsRecoveryRequired}
	sendCalls := 0
	session := &testChatSession{send: func(context.Context, string) (*chat.SendResult, error) {
		sendCalls++
		return &chat.SendResult{Reply: "unexpected"}, nil
	}}
	service := NewService(nil, &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}, session, "model-a", false)
	h := NewHandler(service, testOrigin, backend)
	recorder := request(h, http.MethodPost, "/api/settings", `{"profile_slug":"beta","model":"model-b","revision":1}`, testOrigin)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "settings_recovery_required") || strings.Contains(recorder.Body.String(), "web:") {
		t.Fatalf("recovery response = %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := request(h, http.MethodPost, "/api/messages", `{"request_id":"blocked","content":"hello","profile_slug":"alpha"}`, testOrigin); recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "settings_recovery_required") {
		t.Fatalf("send after recovery required = %d %s", recorder.Code, recorder.Body.String())
	}
	if sendCalls != 0 {
		t.Fatalf("chat session mutated %d times after recovery was required", sendCalls)
	}
}

func TestSettingsSaveValidatesRequestAndSanitizesErrors(t *testing.T) {
	backend := &testSettingsBackend{}
	h := NewHandler(NewService(nil, &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false), testOrigin, backend)
	for _, body := range []string{
		`{"profile_slug":"beta","model":"model-b"}`,
		`{"model":"model-b","revision":1}`,
		`{"profile_slug":"beta","model":"model-b","revision":1,"unexpected":true}`,
	} {
		if recorder := request(h, http.MethodPost, "/api/settings", body, testOrigin); recorder.Code != http.StatusBadRequest {
			t.Errorf("invalid settings body status = %d, want 400: %s", recorder.Code, recorder.Body.String())
		}
	}
	if recorder := requestNoJSON(h, http.MethodPost, "/api/settings", `{"profile_slug":"beta","model":"model-b","revision":1}`, testOrigin); recorder.Code != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON settings status = %d, want 415", recorder.Code)
	}
	if recorder := request(h, http.MethodPost, "/api/settings", `{"profile_slug":"beta","model":"model-b","revision":1}`, "http://evil.example"); recorder.Code != http.StatusForbidden {
		t.Errorf("foreign-origin settings status = %d, want 403", recorder.Code)
	}
	backend.saveErr = ErrSettingsModelNotFound
	if recorder := request(h, http.MethodPost, "/api/settings", `{"profile_slug":"beta","model":"missing","revision":1}`, testOrigin); recorder.Code != http.StatusBadRequest {
		t.Errorf("unknown model status = %d, want 400", recorder.Code)
	}
	backend.saveErr = errors.New("credential-secret provider traceback")
	if recorder := request(h, http.MethodPost, "/api/settings", `{"profile_slug":"beta","model":"model-b","revision":1}`, testOrigin); recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "traceback") {
		t.Errorf("save error was not sanitized: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSaveSettingsWaitsForModelReadLease(t *testing.T) {
	oldProfile := &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}
	session := &trackedChatSession{}
	service := NewService(nil, oldProfile, session, "model-a", false)
	backend := &testSettingsBackend{modelsStarted: make(chan struct{}, 1), releaseModels: make(chan struct{})}

	modelsDone := make(chan struct{})
	go func() {
		_, _ = service.Models(context.Background(), backend, "alpha")
		close(modelsDone)
	}()
	select {
	case <-backend.modelsStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("settings model read did not start")
	}

	saveDone := make(chan error, 1)
	go func() {
		_, err := service.SaveSettings(context.Background(), backend, "beta", "model-b", 1)
		saveDone <- err
	}()
	select {
	case err := <-saveDone:
		t.Fatalf("save completed while model read held its lease: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if session.closes() != 0 {
		t.Fatalf("runtime closed while settings read was active: %d", session.closes())
	}
	close(backend.releaseModels)
	select {
	case <-modelsDone:
	case <-time.After(2 * time.Second):
		t.Fatal("settings model read did not finish")
	}
	if err := <-saveDone; err != nil {
		t.Fatalf("save after model read: %v", err)
	}
	if session.closes() != 1 || service.State().Profile.Slug != "beta" {
		t.Fatalf("post-switch state=%+v old session closes=%d", service.State(), session.closes())
	}
}

func TestSaveSettingsWaitsForHistoryReadLease(t *testing.T) {
	oldProfile := &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}
	session := &trackedChatSession{}
	service := NewService(nil, oldProfile, session, "model-a", false)
	reader := &blockingHistoryReader{started: make(chan struct{}, 1), release: make(chan struct{})}
	service.runtime.messageRepo = reader

	historyDone := make(chan error, 1)
	go func() {
		_, err := service.Messages(context.Background(), nil, 50)
		historyDone <- err
	}()
	select {
	case <-reader.started:
	case <-time.After(2 * time.Second):
		t.Fatal("history read did not start")
	}

	backend := &testSettingsBackend{}
	saveDone := make(chan error, 1)
	go func() {
		_, err := service.SaveSettings(context.Background(), backend, "beta", "model-b", 1)
		saveDone <- err
	}()
	select {
	case err := <-saveDone:
		t.Fatalf("save completed while history read held its lease: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if session.closes() != 0 {
		t.Fatalf("runtime closed while history read was active: %d", session.closes())
	}
	close(reader.release)
	if err := <-historyDone; err != nil {
		t.Fatalf("history read: %v", err)
	}
	if err := <-saveDone; err != nil {
		t.Fatalf("save after history read: %v", err)
	}
	if session.closes() != 1 || service.State().Profile.Slug != "beta" {
		t.Fatalf("post-switch state=%+v old session closes=%d", service.State(), session.closes())
	}
}

func TestSaveSettingsRejectsPendingMemoryExtraction(t *testing.T) {
	service := NewService(nil, &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}, &trackedChatSession{pending: 1}, "model-a", false)
	state, err := service.SaveSettings(context.Background(), &testSettingsBackend{}, "beta", "model-b", 1)
	if !errors.Is(err, ErrSettingsBusy) {
		t.Fatalf("save with pending memory err = %v, want ErrSettingsBusy", err)
	}
	if state.Profile.Slug != "" || service.State().Profile.Slug != "alpha" || service.State().Revision != 1 {
		t.Fatalf("state changed during busy save: result=%+v active=%+v", state, service.State())
	}
}

func TestCloseRuntimeClosesSessionOnce(t *testing.T) {
	session := &trackedChatSession{}
	service := NewService(nil, &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}, session, "model-a", false)
	if err := service.CloseRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.CloseRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if session.closes() != 1 {
		t.Fatalf("session close count = %d, want exactly once", session.closes())
	}
}

func TestCloseRuntimeWaitsForExtractionBeforeClosingDB(t *testing.T) {
	db, err := store.OpenForTesting(t.TempDir() + "/runtime.db")
	if err != nil {
		t.Fatal(err)
	}
	session := &blockingDrainSession{
		trackedChatSession: &trackedChatSession{},
		started:            make(chan struct{}),
		release:            make(chan struct{}),
	}
	service := NewService(db, &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}, session, "model-a", false)
	closeDone := make(chan error, 1)
	go func() { closeDone <- service.CloseRuntime(context.Background()) }()
	select {
	case <-session.started:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime close did not wait for extraction")
	}
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("database closed while extraction was active: %v", err)
	}
	if session.closes() != 0 {
		t.Fatalf("session closed while extraction was active: %d", session.closes())
	}
	close(session.release)
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("CloseRuntime: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runtime close did not finish after extraction")
	}
	if err := db.PingContext(context.Background()); err == nil {
		t.Fatal("database remained open after runtime close")
	}
	if session.closes() != 1 {
		t.Fatalf("session close count = %d, want 1", session.closes())
	}
}

func TestConcurrentSettingsSavesCannotShareRevision(t *testing.T) {
	service := NewService(nil, &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false)
	backend := &testSettingsBackend{saveStarted: make(chan struct{}, 1), releaseSave: make(chan struct{})}
	firstDone := make(chan error, 1)
	go func() {
		_, err := service.SaveSettings(context.Background(), backend, "beta", "model-b", 1)
		firstDone <- err
	}()
	select {
	case <-backend.saveStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first settings save did not reach persistence")
	}
	if _, err := service.SaveSettings(context.Background(), backend, "gamma", "model-c", 1); !errors.Is(err, ErrSettingsBusy) {
		t.Fatalf("concurrent save error = %v, want ErrSettingsBusy", err)
	}
	close(backend.releaseSave)
	if err := <-firstDone; err != nil {
		t.Fatalf("first settings save: %v", err)
	}
	if state := service.State(); state.Profile.Slug != "beta" || state.Revision != 2 {
		t.Fatalf("concurrent saves produced state %+v", state)
	}
}

func TestStateRemainsReadableDuringSlowSettingsPreparation(t *testing.T) {
	service := NewService(nil, &store.Profile{ID: "alpha", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false)
	backend := &testSettingsBackend{saveStarted: make(chan struct{}, 1), releaseSave: make(chan struct{})}
	saveDone := make(chan error, 1)
	go func() {
		_, err := service.SaveSettings(context.Background(), backend, "beta", "model-b", 1)
		saveDone <- err
	}()
	select {
	case <-backend.saveStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("settings preparation did not start")
	}
	stateDone := make(chan StateDTO, 1)
	go func() { stateDone <- service.State() }()
	select {
	case state := <-stateDone:
		if state.Profile.Slug != "alpha" || state.Revision != 1 {
			t.Fatalf("preparation exposed uncommitted state: %+v", state)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("state read blocked behind slow settings preparation")
	}
	close(backend.releaseSave)
	if err := <-saveDone; err != nil {
		t.Fatalf("settings save: %v", err)
	}
}

func TestMessageRejectsStaleProfile(t *testing.T) {
	db := newWebTestDB(t)
	profile := &store.Profile{ID: "p1", Name: "Artist", Slug: "artist"}
	sendCalls := 0
	session := &testChatSession{send: func(context.Context, string) (*chat.SendResult, error) {
		sendCalls++
		return &chat.SendResult{Reply: "unexpected"}, nil
	}}
	h := NewHandler(NewService(db, profile, session, "model-a", false), testOrigin)
	body := `{"request_id":"stale-profile","content":"hello","profile_slug":"old-profile"}`
	if recorder := request(h, http.MethodPost, "/api/messages", body, testOrigin); recorder.Code != http.StatusConflict {
		t.Fatalf("stale-profile send status = %d, want 409: %s", recorder.Code, recorder.Body.String())
	}
	if sendCalls != 0 {
		t.Fatalf("stale profile send called session %d times", sendCalls)
	}
}

type testSettingsBackend struct {
	modelsErr     error
	modelsStarted chan struct{}
	releaseModels chan struct{}
	saveStarted   chan struct{}
	releaseSave   chan struct{}
	saveErr       error
	nextSession   ChatSession
	selection     *ModelSettingsSelection
}

func (*testSettingsBackend) Profiles(context.Context) ([]SettingsProfileDTO, error) {
	return []SettingsProfileDTO{{Slug: "artist", Name: "Artist"}}, nil
}

func (b *testSettingsBackend) Models(_ context.Context, slug string) (ProfileModelsDTO, error) {
	if b.modelsStarted != nil {
		b.modelsStarted <- struct{}{}
	}
	if b.releaseModels != nil {
		<-b.releaseModels
	}
	if b.modelsErr != nil {
		return ProfileModelsDTO{}, b.modelsErr
	}
	return ProfileModelsDTO{ProfileSlug: slug, SavedModel: "saved-model", Models: []string{"model-a", "model-b"}}, nil
}

func (b *testSettingsBackend) Prepare(_ context.Context, current ProfileRuntime, slug, model string) (PreparedSettings, error) {
	if b.saveStarted != nil {
		b.saveStarted <- struct{}{}
	}
	if b.releaseSave != nil {
		<-b.releaseSave
	}
	if b.saveErr != nil {
		return nil, b.saveErr
	}
	return &testPreparedSettings{runtime: NewProfileRuntime(nil, &store.Profile{ID: slug, Name: slug, Slug: slug}, b.nextSession, model, false)}, nil
}

func (b *testSettingsBackend) PrepareModels(ctx context.Context, current ProfileRuntime, slug string, selection ModelSettingsSelection) (PreparedSettings, error) {
	copy := selection
	if selection.EmbeddingsModel != nil {
		value := *selection.EmbeddingsModel
		copy.EmbeddingsModel = &value
	}
	if selection.ExtractorModel != nil {
		value := *selection.ExtractorModel
		copy.ExtractorModel = &value
	}
	b.selection = &copy
	return b.Prepare(ctx, current, slug, selection.Model)
}

type testPreparedSettings struct {
	runtime ProfileRuntime
	err     error
}

func (p *testPreparedSettings) Commit(context.Context) (ProfileRuntime, error) {
	return p.runtime, p.err
}
func (*testPreparedSettings) Abort() {}

type trackedChatSession struct {
	mu         sync.Mutex
	pending    int
	closeCount int
}

type blockingDrainSession struct {
	*trackedChatSession
	started chan struct{}
	release chan struct{}
}

func (s *blockingDrainSession) WaitForPendingNotes(ctx context.Context) error {
	close(s.started)
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type blockingHistoryReader struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingHistoryReader) ListHistoryByProfile(context.Context, string, *store.MessageHistoryCursor, int) (store.MessageHistoryPage, error) {
	r.started <- struct{}{}
	<-r.release
	return store.MessageHistoryPage{Messages: []*store.Message{}}, nil
}

func (*trackedChatSession) Send(context.Context, string) (*chat.SendResult, error) {
	return &chat.SendResult{Reply: "reply"}, nil
}
func (s *trackedChatSession) PendingNotes() int { return s.pending }
func (s *trackedChatSession) Close(context.Context) {
	s.mu.Lock()
	s.closeCount++
	s.mu.Unlock()
}
func (s *trackedChatSession) closes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeCount
}
