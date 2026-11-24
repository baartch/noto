package app

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/security"
	"noto/internal/store"
	"noto/internal/voice"
	"noto/internal/web"
)

type appVoiceTestModels struct {
	status          voice.ModelStatus
	transcriber     voice.Transcriber
	downloadStarted chan struct{}
	downloadDone    chan error

	transcribeCalls atomic.Int32
	downloadCalls   atomic.Int32
	deleteCalls     atomic.Int32
}

func (m *appVoiceTestModels) Status() voice.ModelStatus { return m.status }
func (m *appVoiceTestModels) LocalTranscriber() (voice.Transcriber, error) {
	if !m.status.Available {
		return nil, voice.ErrLocalRuntimeUnavailable
	}
	if !m.status.Installed {
		return nil, voice.ErrLocalModelNotInstalled
	}
	return appVoiceTestTranscriber(func(ctx context.Context, audio []byte) (string, error) {
		m.transcribeCalls.Add(1)
		return m.transcriber.Transcribe(ctx, audio)
	}), nil
}
func (m *appVoiceTestModels) Download(ctx context.Context) error {
	m.downloadCalls.Add(1)
	if m.downloadStarted != nil {
		close(m.downloadStarted)
	}
	if m.downloadDone != nil {
		<-ctx.Done()
		m.downloadDone <- ctx.Err()
	}
	return nil
}
func (m *appVoiceTestModels) Delete(context.Context) error {
	m.deleteCalls.Add(1)
	return nil
}

type appVoiceTestTranscriber func(context.Context, []byte) (string, error)

func (f appVoiceTestTranscriber) Transcribe(ctx context.Context, audio []byte) (string, error) {
	return f(ctx, audio)
}

func TestWebVoiceBackendReadinessAndDispatchStayProfileScoped(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	t.Setenv("NOTO_API_KEY", "")
	t.Setenv("NOTO_ENDPOINT", "")
	t.Setenv("NOTO_MODEL", "")
	ctx := context.Background()
	profiles := profile.NewService(nil)
	alpha, err := profiles.Create(ctx, "Alpha")
	if err != nil {
		t.Fatal(err)
	}
	beta, err := profiles.Create(ctx, "Beta")
	if err != nil {
		t.Fatal(err)
	}
	gamma, err := profiles.Create(ctx, "Gamma")
	if err != nil {
		t.Fatal(err)
	}
	if err := addVoiceTestProvider(ctx, beta, "https://openrouter.ai/api/v1", "beta-secret"); err != nil {
		t.Fatal(err)
	}
	if err := addVoiceTestProvider(ctx, gamma, "https://api.openai.com/v1", "gamma-secret"); err != nil {
		t.Fatal(err)
	}
	localCalls := atomic.Int32{}
	providerCalls := atomic.Int32{}
	openRouterDiscoveryCalls := atomic.Int32{}
	models := &appVoiceTestModels{
		status: voice.ModelStatus{Available: true, Installed: true},
		transcriber: appVoiceTestTranscriber(func(context.Context, []byte) (string, error) {
			localCalls.Add(1)
			return "local transcript", nil
		}),
	}
	backend := &webVoiceBackend{
		profiles: profiles,
		openDB:   openProfileDB,
		models:   models,
		listTranscriptionModels: func(_ context.Context, cfg provider.Config) ([]string, error) {
			if cfg.Endpoint == "https://api.openai.com/v1" {
				if cfg.APIKey != "gamma-secret" {
					t.Errorf("unsupported provider config = %+v", cfg)
				}
				return nil, voice.ErrTranscriptionModelsUnsupported
			}
			if cfg.Endpoint != "https://openrouter.ai/api/v1" || cfg.APIKey != "beta-secret" {
				t.Errorf("speech model discovery config = %+v", cfg)
			}
			openRouterDiscoveryCalls.Add(1)
			return []string{"whisper-1"}, nil
		},
		newProviderTranscriber: func(cfg provider.Config, model string) voice.Transcriber {
			if cfg.APIKey != "beta-secret" || model != "whisper-1" {
				t.Errorf("provider transcription config = %+v model=%q", cfg, model)
			}
			providerCalls.Add(1)
			return appVoiceTestTranscriber(func(context.Context, []byte) (string, error) { return "provider transcript", nil })
		},
	}
	backend.bindContext(ctx)

	if got, err := backend.Models(ctx, beta.Slug); err != nil || len(got) != 1 || got[0] != "whisper-1" {
		t.Fatalf("provider transcription models = %v, err=%v", got, err)
	}

	if err := backend.Save(ctx, alpha.Slug, store.VoiceSettings{Provider: "local"}, 1); err != nil {
		t.Fatalf("save incomplete local selection: %v", err)
	}
	dto, err := backend.Get(ctx, alpha.Slug)
	if err != nil || dto.Ready || dto.Reason == "" {
		t.Fatalf("blank local selection = %+v, err=%v; want not ready", dto, err)
	}
	if _, err := backend.Transcribe(ctx, alpha.Slug, makeAppVoiceWAV(2)); !errors.Is(err, web.ErrVoiceNotReady) || localCalls.Load() != 0 {
		t.Fatalf("blank local dispatch err=%v local calls=%d", err, localCalls.Load())
	}
	if err := backend.Save(ctx, alpha.Slug, store.VoiceSettings{Provider: "local", Model: voice.LocalModelID}, 2); err != nil {
		t.Fatalf("save local selection: %v", err)
	}
	dto, err = backend.Get(ctx, alpha.Slug)
	if err != nil || !dto.Ready {
		t.Fatalf("installed local selection = %+v, err=%v", dto, err)
	}
	text, err := backend.Transcribe(ctx, alpha.Slug, makeAppVoiceWAV(2))
	if err != nil || text != "local transcript" || localCalls.Load() != 1 {
		t.Fatalf("local dispatch = %q, err=%v calls=%d", text, err, localCalls.Load())
	}
	models.status.Error = "voice: could not open local speech model: transient failure"
	dto, err = backend.Get(ctx, alpha.Slug)
	if err != nil || !dto.Ready {
		t.Errorf("transient local model error readiness = %+v, err=%v; want ready for retry", dto, err)
	}
	text, err = backend.Transcribe(ctx, alpha.Slug, makeAppVoiceWAV(2))
	if err != nil || text != "local transcript" || localCalls.Load() != 2 {
		t.Errorf("local retry after open error = %q, err=%v calls=%d; want a second dispatch", text, err, localCalls.Load())
	}
	models.status.Available = false
	dto, err = backend.Get(ctx, alpha.Slug)
	if err != nil || dto.Ready {
		t.Fatalf("unsupported local runtime = %+v, err=%v; want not ready", dto, err)
	}
	if _, err := backend.Transcribe(ctx, alpha.Slug, makeAppVoiceWAV(2)); !errors.Is(err, voice.ErrLocalModelNotInstalled) || localCalls.Load() != 2 {
		t.Fatalf("unsupported local dispatch err=%v local calls=%d", err, localCalls.Load())
	}
	models.status.Available = true

	if err := backend.Save(ctx, beta.Slug, store.VoiceSettings{Provider: "provider", Model: "chat-model"}, 3); err != nil {
		t.Fatalf("save mismatched provider model: %v", err)
	}
	dto, err = backend.Get(ctx, beta.Slug)
	if err != nil || dto.Ready {
		t.Fatalf("unverified provider selection = %+v, err=%v; want not ready", dto, err)
	}
	if _, err := backend.Transcribe(ctx, beta.Slug, makeAppVoiceWAV(2)); !errors.Is(err, web.ErrVoiceNotReady) || providerCalls.Load() != 0 {
		t.Fatalf("unverified provider dispatch err=%v factory calls=%d", err, providerCalls.Load())
	}
	if err := backend.Save(ctx, beta.Slug, store.VoiceSettings{Provider: "provider", Model: "whisper-1"}, 4); err != nil {
		t.Fatalf("save verified provider model: %v", err)
	}
	dto, err = backend.Get(ctx, beta.Slug)
	if err != nil || !dto.Ready {
		t.Fatalf("verified provider selection = %+v, err=%v", dto, err)
	}
	text, err = backend.Transcribe(ctx, beta.Slug, makeAppVoiceWAV(2))
	if err != nil || text != "provider transcript" || providerCalls.Load() != 1 {
		t.Fatalf("provider dispatch = %q, err=%v factory calls=%d", text, err, providerCalls.Load())
	}
	if got := openRouterDiscoveryCalls.Load(); got != 1 {
		t.Fatalf("OpenRouter discovery calls = %d, want 1 across Models, Get and Transcribe", got)
	}
	if err := backend.Save(ctx, gamma.Slug, store.VoiceSettings{Provider: "provider", Model: "whisper-1"}, 5); err != nil {
		t.Fatalf("save unsupported provider selection: %v", err)
	}
	dto, err = backend.Get(ctx, gamma.Slug)
	if err != nil || dto.Ready || dto.Reason == "" {
		t.Fatalf("unsupported provider selection = %+v, err=%v; want not ready", dto, err)
	}
	if _, err := backend.Transcribe(ctx, gamma.Slug, makeAppVoiceWAV(2)); !errors.Is(err, voice.ErrTranscriptionModelsUnsupported) || providerCalls.Load() != 1 {
		t.Fatalf("unsupported provider dispatch err=%v factory calls=%d", err, providerCalls.Load())
	}
	encoded, err := json.Marshal(dto)
	if err != nil || strings.Contains(string(encoded), "beta-secret") {
		t.Fatalf("voice DTO leaked profile credential: %s, err=%v", encoded, err)
	}
}

func TestWebVoiceDownloadOutlivesRequestAndCancelsAtShutdown(t *testing.T) {
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	models := &appVoiceTestModels{
		status:          voice.ModelStatus{Available: true},
		downloadStarted: make(chan struct{}),
		downloadDone:    make(chan error, 1),
	}
	backend := &webVoiceBackend{models: models}
	backend.bindContext(serverCtx)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	if err := backend.Download(requestCtx); err != nil {
		t.Fatalf("Download() = %v", err)
	}
	select {
	case <-models.downloadStarted:
	case <-time.After(time.Second):
		t.Fatal("background download did not start")
	}
	if err := backend.Download(context.Background()); !errors.Is(err, web.ErrVoiceOperationConflict) {
		t.Fatalf("duplicate Download() error = %v, want operation conflict", err)
	}
	if err := backend.Delete(context.Background()); !errors.Is(err, web.ErrVoiceOperationConflict) {
		t.Fatalf("Delete() during download error = %v, want operation conflict", err)
	}
	cancelRequest()
	select {
	case err := <-models.downloadDone:
		t.Fatalf("request cancellation stopped server-owned download: %v", err)
	default:
	}
	cancelServer()
	select {
	case err := <-models.downloadDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("server shutdown download error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server shutdown did not cancel background download")
	}
	if err := backend.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() = %v", err)
	}
}

func TestTranscriptionModelCacheReusesResultsWithoutSharingCredentialEntries(t *testing.T) {
	cache := newTranscriptionModelCache()
	calls := 0
	fetch := func(_ context.Context, cfg provider.Config) ([]string, error) {
		calls++
		return []string{cfg.APIKey}, nil
	}
	cfg := provider.Config{Endpoint: "https://openrouter.ai/api/v1", APIKey: "profile-a-secret"}

	got, err := cache.get(context.Background(), cfg, fetch)
	if err != nil || len(got) != 1 || got[0] != cfg.APIKey {
		t.Fatalf("first cache lookup = %v, err=%v", got, err)
	}
	got[0] = "mutated"
	got, err = cache.get(context.Background(), cfg, fetch)
	if err != nil || len(got) != 1 || got[0] != cfg.APIKey {
		t.Fatalf("cached lookup = %v, err=%v; want independent cached copy", got, err)
	}
	otherCfg := cfg
	otherCfg.APIKey = "profile-b-secret"
	got, err = cache.get(context.Background(), otherCfg, fetch)
	if err != nil || len(got) != 1 || got[0] != otherCfg.APIKey {
		t.Fatalf("other credential lookup = %v, err=%v", got, err)
	}
	if calls != 2 {
		t.Fatalf("fetch calls = %d, want 2 (one per credential)", calls)
	}
}

func TestTranscriptionModelCacheExpiresAndRetriesErrors(t *testing.T) {
	cache := newTranscriptionModelCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	calls := 0
	fetch := func(context.Context, provider.Config) ([]string, error) {
		calls++
		switch calls {
		case 1:
			return nil, errors.New("temporary lookup failure")
		case 2:
			return []string{"speech-v1"}, nil
		default:
			return []string{"speech-v2"}, nil
		}
	}
	cfg := provider.Config{Endpoint: "https://openrouter.ai/api/v1"}

	if _, err := cache.get(context.Background(), cfg, fetch); err == nil {
		t.Fatal("first lookup succeeded; want temporary failure")
	}
	if got, err := cache.get(context.Background(), cfg, fetch); err != nil || len(got) != 1 || got[0] != "speech-v1" {
		t.Fatalf("retry lookup = %v, err=%v", got, err)
	}
	if got, err := cache.get(context.Background(), cfg, fetch); err != nil || len(got) != 1 || got[0] != "speech-v1" {
		t.Fatalf("cached lookup = %v, err=%v", got, err)
	}
	if calls != 2 {
		t.Fatalf("fetch calls before expiry = %d, want 2", calls)
	}

	now = now.Add(transcriptionModelCacheTTL)
	if got, err := cache.get(context.Background(), cfg, fetch); err != nil || len(got) != 1 || got[0] != "speech-v2" {
		t.Fatalf("expired lookup = %v, err=%v; want refreshed models", got, err)
	}
	if calls != 3 {
		t.Fatalf("fetch calls after expiry = %d, want 3", calls)
	}
}

func addVoiceTestProvider(ctx context.Context, p *store.Profile, endpoint, apiKey string) error {
	db, err := openProfileDB(p.Slug)
	if err != nil {
		return err
	}
	defer db.Close()
	passphrase, err := security.MachinePassphrase()
	if err != nil {
		return err
	}
	credential, err := security.Encrypt(apiKey, passphrase)
	if err != nil {
		return err
	}
	return store.NewProviderConfigRepo(db).Upsert(ctx, &store.ProviderConfig{
		ID: "provider-" + p.Slug, ProfileID: p.ID, ProviderType: "openai_compatible",
		Endpoint: endpoint, ActiveModel: "chat-model", CredentialRef: credential, IsActive: true,
	})
}

func makeAppVoiceWAV(payloadBytes int) []byte {
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
