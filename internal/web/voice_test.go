package web

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"noto/internal/store"
	"noto/internal/voice"
)

type voiceSaveCall struct {
	profileSlug string
	settings    store.VoiceSettings
	revision    uint64
}

type voiceTranscribeCall struct {
	profileSlug string
	audio       []byte
}

type testVoiceBackend struct {
	mu sync.Mutex

	getDTO        VoiceDTO
	getErr        error
	saveErr       error
	models        []string
	modelsErr     error
	modelsStarted chan struct{}
	releaseModels chan struct{}
	transcribeErr error
	transcript    string
	status        voice.ModelStatus
	getStarted    chan struct{}
	releaseGet    chan struct{}

	getCalls        []string
	saveCalls       []voiceSaveCall
	modelCalls      []string
	transcribeCalls []voiceTranscribeCall
	downloadCalls   int
	deleteCalls     int
}

func (b *testVoiceBackend) Get(_ context.Context, profileSlug string) (VoiceDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.getCalls = append(b.getCalls, profileSlug)
	if b.getStarted != nil {
		close(b.getStarted)
		<-b.releaseGet
	}
	dto := b.getDTO
	dto.ProfileSlug = profileSlug
	return dto, b.getErr
}

func (b *testVoiceBackend) Save(_ context.Context, profileSlug string, settings store.VoiceSettings, revision uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.saveCalls = append(b.saveCalls, voiceSaveCall{profileSlug: profileSlug, settings: settings, revision: revision})
	return b.saveErr
}

func (b *testVoiceBackend) Models(_ context.Context, profileSlug string) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.modelCalls = append(b.modelCalls, profileSlug)
	if b.modelsStarted != nil {
		close(b.modelsStarted)
		<-b.releaseModels
	}
	return append([]string(nil), b.models...), b.modelsErr
}

func (b *testVoiceBackend) Transcribe(_ context.Context, profileSlug string, audio []byte) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transcribeCalls = append(b.transcribeCalls, voiceTranscribeCall{profileSlug: profileSlug, audio: append([]byte(nil), audio...)})
	return b.transcript, b.transcribeErr
}

func (b *testVoiceBackend) Download(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.downloadCalls++
	return nil
}

func (b *testVoiceBackend) Delete(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deleteCalls++
	return nil
}

func (b *testVoiceBackend) ModelStatus() voice.ModelStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status
}

func (b *testVoiceBackend) saveCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.saveCalls)
}

func (b *testVoiceBackend) transcribeCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.transcribeCalls)
}

func TestVoiceSettingsAndModelRoutes(t *testing.T) {
	backend := &testVoiceBackend{
		getDTO:     VoiceDTO{Provider: "provider", Model: "whisper-1", Ready: true},
		models:     []string{"whisper-1", "parakeet-stt"},
		status:     voice.ModelStatus{Available: false},
		transcript: "dictated words",
	}
	service := NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "", false)
	h := NewHandlerWithVoice(service, testOrigin, nil, backend)

	get := request(h, http.MethodGet, "/api/voice?profile_slug=alpha", "", "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET /api/voice status = %d: %s", get.Code, get.Body.String())
	}
	var dto VoiceDTO
	if err := json.Unmarshal(get.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.ProfileSlug != "alpha" || dto.Provider != "provider" || dto.Model != "whisper-1" || !dto.Ready || dto.Reason != "" {
		t.Fatalf("voice DTO = %+v", dto)
	}
	if strings.Contains(get.Body.String(), "api_key") || strings.Contains(get.Body.String(), "credential") {
		t.Fatalf("voice DTO leaked credential data: %s", get.Body.String())
	}

	models := request(h, http.MethodGet, "/api/voice/models?profile_slug=alpha", "", "")
	if models.Code != http.StatusOK || !strings.Contains(models.Body.String(), `"models":["whisper-1","parakeet-stt"]`) {
		t.Fatalf("GET /api/voice/models = %d %s", models.Code, models.Body.String())
	}

	body := `{"profile_slug":"alpha","provider":"provider","model":"whisper-1","revision":1}`
	saved := request(h, http.MethodPost, "/api/voice", body, testOrigin)
	if saved.Code != http.StatusOK || backend.saveCallCount() != 1 || service.State().Revision != 2 {
		t.Fatalf("POST /api/voice = %d %s; saves=%d state=%+v", saved.Code, saved.Body.String(), backend.saveCallCount(), service.State())
	}
	nonActive := request(h, http.MethodPost, "/api/voice", `{"profile_slug":"beta","provider":"provider","model":"whisper-1","revision":2}`, testOrigin)
	if nonActive.Code != http.StatusConflict || backend.saveCallCount() != 1 {
		t.Fatalf("non-active profile voice save = %d %s; saves=%d", nonActive.Code, nonActive.Body.String(), backend.saveCallCount())
	}
	stale := request(h, http.MethodPost, "/api/voice", body, testOrigin)
	if stale.Code != http.StatusConflict || backend.saveCallCount() != 1 {
		t.Fatalf("stale POST /api/voice = %d %s; saves=%d", stale.Code, stale.Body.String(), backend.saveCallCount())
	}
}

func TestVoiceSaveValidationFailureDoesNotAdvanceRevision(t *testing.T) {
	backend := &testVoiceBackend{saveErr: ErrVoiceModelUnverified}
	service := NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false)
	h := NewHandlerWithVoice(service, testOrigin, nil, backend)
	recorder := request(h, http.MethodPost, "/api/voice", `{"profile_slug":"alpha","provider":"provider","model":"unknown-model","revision":1}`, testOrigin)
	if recorder.Code != http.StatusUnprocessableEntity || service.State().Revision != 1 {
		t.Fatalf("invalid voice save = %d %s; revision=%d, want 422 and unchanged revision 1", recorder.Code, recorder.Body.String(), service.State().Revision)
	}
}

func TestVoiceGetKeepsTheRevisionCapturedBeforeProfileIO(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	backend := &testVoiceBackend{
		getDTO:     VoiceDTO{Provider: "local", Model: "parakeet-tdt-0.6b-v3-int8"},
		getStarted: make(chan struct{}),
		releaseGet: release,
	}
	service := NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false)
	h := NewHandlerWithVoice(service, testOrigin, nil, backend)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(recorder, newAPIRequest(context.Background(), http.MethodGet, "/api/voice?profile_slug=alpha", "", ""))
		close(done)
	}()
	select {
	case <-backend.getStarted:
	case <-time.After(time.Second):
		t.Fatal("voice GET did not reach backend")
	}
	if _, err := service.SaveSettings(context.Background(), &testSettingsBackend{}, "beta", "model-b", 1); err != nil {
		t.Fatalf("switch active profile during voice GET: %v", err)
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("voice GET did not finish")
	}
	var dto VoiceDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.ProfileSlug != "alpha" || dto.Revision != 1 || service.State().Revision != 2 {
		t.Fatalf("stale voice response = %+v, current revision=%d", dto, service.State().Revision)
	}
}

func TestVoiceModelsResponseKeepsTheRevisionCapturedBeforeProfileIO(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	backend := &testVoiceBackend{
		models:        []string{"whisper-1"},
		modelsStarted: make(chan struct{}),
		releaseModels: release,
	}
	service := NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false)
	h := NewHandlerWithVoice(service, testOrigin, nil, backend)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(recorder, newAPIRequest(context.Background(), http.MethodGet, "/api/voice/models?profile_slug=alpha", "", ""))
		close(done)
	}()
	select {
	case <-backend.modelsStarted:
	case <-time.After(time.Second):
		t.Fatal("voice models GET did not reach backend")
	}
	if _, err := service.SaveSettings(context.Background(), &testSettingsBackend{}, "beta", "model-b", 1); err != nil {
		t.Fatalf("switch active profile during models GET: %v", err)
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("voice models GET did not finish")
	}
	var dto VoiceModelsDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.ProfileSlug != "alpha" || dto.Revision != 1 || service.State().Revision != 2 {
		t.Fatalf("stale voice models response = %+v, current revision=%d", dto, service.State().Revision)
	}
}

func TestVoiceTranscriptionEnforcesMIMESizeAndProfile(t *testing.T) {
	backend := &testVoiceBackend{transcript: "spoken words", status: voice.ModelStatus{Available: true, Installed: true}}
	service := NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "", false)
	h := NewHandlerWithVoice(service, testOrigin, nil, backend)
	valid := makeVoiceWAV(2)

	wrongMIME := requestNoJSON(h, http.MethodPost, "/api/voice/transcriptions?profile_slug=alpha", string(valid), testOrigin)
	if wrongMIME.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong MIME status = %d, want 415: %s", wrongMIME.Code, wrongMIME.Body.String())
	}
	if backend.transcribeCallCount() != 0 {
		t.Fatal("wrong MIME was dispatched")
	}

	wrongProfile := requestAudio(h, "/api/voice/transcriptions?profile_slug=beta", valid, testOrigin)
	if wrongProfile.Code != http.StatusConflict || backend.transcribeCallCount() != 0 {
		t.Fatalf("stale profile transcription = %d %s; calls=%d", wrongProfile.Code, wrongProfile.Body.String(), backend.transcribeCallCount())
	}

	tooLarge := requestAudio(h, "/api/voice/transcriptions?profile_slug=alpha", makeVoiceWAV(maxVoiceWAVBytes-44+2), testOrigin)
	if tooLarge.Code != http.StatusRequestEntityTooLarge || backend.transcribeCallCount() != 0 {
		t.Fatalf("oversized transcription = %d %s; calls=%d", tooLarge.Code, tooLarge.Body.String(), backend.transcribeCallCount())
	}
	atLimit := requestAudio(h, "/api/voice/transcriptions?profile_slug=alpha", makeVoiceWAV(maxVoiceWAVBytes-44), testOrigin)
	if atLimit.Code != http.StatusOK || backend.transcribeCallCount() != 1 {
		t.Fatalf("five-minute WAV boundary = %d %s; calls=%d", atLimit.Code, atLimit.Body.String(), backend.transcribeCallCount())
	}

	validResponse := requestAudio(h, "/api/voice/transcriptions?profile_slug=alpha", valid, testOrigin)
	if validResponse.Code != http.StatusOK || !strings.Contains(validResponse.Body.String(), `"text":"spoken words"`) || backend.transcribeCallCount() != 2 {
		t.Fatalf("valid transcription = %d %s; calls=%d", validResponse.Code, validResponse.Body.String(), backend.transcribeCallCount())
	}
}

func TestVoiceTranscriptionRejectsFormerActiveProfile(t *testing.T) {
	backend := &testVoiceBackend{transcript: "must not run"}
	service := NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "model-a", false)
	if _, err := service.SaveSettings(context.Background(), &testSettingsBackend{}, "beta", "model-b", 1); err != nil {
		t.Fatalf("switch to beta profile: %v", err)
	}
	h := NewHandlerWithVoice(service, testOrigin, nil, backend)
	recorder := requestAudio(h, "/api/voice/transcriptions?profile_slug=alpha", makeVoiceWAV(2), testOrigin)
	if recorder.Code != http.StatusConflict || backend.transcribeCallCount() != 0 {
		t.Fatalf("former active profile transcription = %d %s; backend calls=%d", recorder.Code, recorder.Body.String(), backend.transcribeCallCount())
	}
}

func TestVoiceRoutesRejectMethodsOriginsAndHosts(t *testing.T) {
	backend := &testVoiceBackend{status: voice.ModelStatus{Available: true}}
	h := NewHandlerWithVoice(NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "", false), testOrigin, nil, backend)
	for _, tc := range []struct {
		path  string
		allow string
	}{
		{path: "/api/voice?profile_slug=alpha", allow: "GET, POST"},
		{path: "/api/voice/models?profile_slug=alpha", allow: "GET"},
		{path: "/api/voice/transcriptions?profile_slug=alpha", allow: "POST"},
		{path: "/api/voice/local-model", allow: "GET, DELETE"},
		{path: "/api/voice/local-model/download", allow: "POST"},
	} {
		recorder := request(h, http.MethodPut, tc.path, "", testOrigin)
		if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != tc.allow {
			t.Errorf("PUT %s = %d Allow=%q, want 405 %q", tc.path, recorder.Code, recorder.Header().Get("Allow"), tc.allow)
		}
	}
	if recorder := request(h, http.MethodPost, "/api/voice", `{"profile_slug":"alpha","revision":1}`, ""); recorder.Code != http.StatusForbidden {
		t.Errorf("POST without Origin = %d, want 403", recorder.Code)
	}
	if recorder := request(h, http.MethodDelete, "/api/voice/local-model", "", ""); recorder.Code != http.StatusForbidden {
		t.Errorf("DELETE without Origin = %d, want 403", recorder.Code)
	}
	if recorder := requestAudio(h, "/api/voice/transcriptions?profile_slug=alpha", makeVoiceWAV(2), "http://evil.example"); recorder.Code != http.StatusForbidden {
		t.Errorf("transcription with foreign Origin = %d, want 403", recorder.Code)
	}
	if recorder := requestWithHost(h, http.MethodGet, "/api/voice/local-model", "", "", "attacker.example"); recorder.Code != http.StatusMisdirectedRequest {
		t.Errorf("foreign host = %d, want 421", recorder.Code)
	}
	if backend.downloadCalls != 0 || backend.deleteCalls != 0 {
		t.Fatalf("security failures dispatched operations: download=%d delete=%d", backend.downloadCalls, backend.deleteCalls)
	}
}

func TestVoiceLocalModelConflictsAndErrorRedaction(t *testing.T) {
	backend := &testVoiceBackend{status: voice.ModelStatus{Available: true, Error: "secret=/home/alice/model"}}
	h := NewHandlerWithVoice(NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "", false), testOrigin, nil, backend)

	status := request(h, http.MethodGet, "/api/voice/local-model", "", "")
	if status.Code != http.StatusOK || strings.Contains(status.Body.String(), "/home/alice") || strings.Contains(status.Body.String(), "secret=") {
		t.Fatalf("local model status leaked internal error: %d %s", status.Code, status.Body.String())
	}

	backend.status.Downloading = true
	if recorder := request(h, http.MethodPost, "/api/voice/local-model/download", "", testOrigin); recorder.Code != http.StatusConflict || backend.downloadCalls != 0 {
		t.Errorf("download conflict = %d %s; calls=%d", recorder.Code, recorder.Body.String(), backend.downloadCalls)
	}
	backend.status.Downloading = false
	backend.status.Deleting = true
	if recorder := request(h, http.MethodDelete, "/api/voice/local-model", "", testOrigin); recorder.Code != http.StatusConflict || backend.deleteCalls != 0 {
		t.Errorf("delete conflict = %d %s; calls=%d", recorder.Code, recorder.Body.String(), backend.deleteCalls)
	}
}

func TestVoiceBackendErrorsAreRedacted(t *testing.T) {
	backend := &testVoiceBackend{getErr: errors.New("credential=top-secret provider trace")}
	h := NewHandlerWithVoice(NewService(nil, &store.Profile{ID: "alpha-id", Name: "Alpha", Slug: "alpha"}, nil, "", false), testOrigin, nil, backend)
	recorder := request(h, http.MethodGet, "/api/voice?profile_slug=alpha", "", "")
	if recorder.Code != http.StatusBadGateway || strings.Contains(recorder.Body.String(), "top-secret") || strings.Contains(recorder.Body.String(), "provider trace") {
		t.Fatalf("backend error was not redacted: %d %s", recorder.Code, recorder.Body.String())
	}
	backend.getErr = nil
	backend.transcribeErr = errors.New("audio bytes and credential=top-secret")
	transcription := requestAudio(h, "/api/voice/transcriptions?profile_slug=alpha", makeVoiceWAV(2), testOrigin)
	if transcription.Code != http.StatusBadGateway || strings.Contains(transcription.Body.String(), "top-secret") || strings.Contains(transcription.Body.String(), "audio bytes") {
		t.Fatalf("transcription error was not redacted: %d %s", transcription.Code, transcription.Body.String())
	}
	backend.modelsErr = voice.ErrTranscriptionModelsUnsupported
	models := request(h, http.MethodGet, "/api/voice/models?profile_slug=alpha", "", "")
	if models.Code != http.StatusServiceUnavailable || !strings.Contains(models.Body.String(), "voice_models_unsupported") || strings.Contains(models.Body.String(), "voice:") {
		t.Fatalf("unsupported models response = %d %s", models.Code, models.Body.String())
	}
	if recorder := request(NewHandler(nil, testOrigin), http.MethodGet, "/api/voice?profile_slug=alpha", "", ""); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing voice backend status = %d, want 503: %s", recorder.Code, recorder.Body.String())
	}
}

func requestAudio(h http.Handler, path string, audio []byte, origin string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := newAPIRequest(context.Background(), http.MethodPost, path, string(audio), origin)
	req.Header.Set("Content-Type", "audio/wav")
	h.ServeHTTP(recorder, req)
	return recorder
}

func makeVoiceWAV(payloadBytes int) []byte {
	data := make([]byte, 44+payloadBytes)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], 16000)
	binary.LittleEndian.PutUint32(data[28:32], 32000)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(payloadBytes))
	return data
}
