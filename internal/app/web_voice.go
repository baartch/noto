package app

import (
	"context"
	"errors"
	"slices"
	"sync"

	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/voice"
	"noto/internal/web"
)

type voiceModelLifecycle interface {
	Status() voice.ModelStatus
	LocalTranscriber() (voice.Transcriber, error)
	Download(context.Context) error
	Delete(context.Context) error
}

type webVoiceBackend struct {
	profiles *profile.Service
	openDB   func(string) (*store.DB, error)
	models   voiceModelLifecycle

	listTranscriptionModels func(context.Context, provider.Config) ([]string, error)
	newProviderTranscriber  func(provider.Config, string) voice.Transcriber

	mu            sync.Mutex
	serviceCtx    context.Context
	operation     string
	operationDone chan struct{}
}

func newWebVoiceBackend(ctx context.Context, models voiceModelLifecycle) *webVoiceBackend {
	b := &webVoiceBackend{
		profiles:                profile.NewService(nil),
		openDB:                  openProfileDB,
		models:                  models,
		listTranscriptionModels: voice.ListTranscriptionModels,
		newProviderTranscriber:  voice.NewProviderTranscriber,
	}
	b.bindContext(ctx)
	return b
}

func (b *webVoiceBackend) bindContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	b.mu.Lock()
	b.serviceCtx = ctx
	b.mu.Unlock()
}

func (b *webVoiceBackend) Get(ctx context.Context, slug string) (web.VoiceDTO, error) {
	loaded, err := b.loadVoiceProfile(ctx, slug)
	if err != nil {
		return web.VoiceDTO{}, err
	}
	status := b.ModelStatus()
	dto := web.VoiceDTO{ProfileSlug: loaded.profile.Slug, Provider: loaded.settings.Provider, Model: loaded.settings.Model, ModelStatus: status}
	switch loaded.settings.Provider {
	case "":
		dto.Reason = "Choose local speech or a configured provider in Voice settings."
	case web.VoiceProviderLocal:
		switch {
		case loaded.settings.Model != voice.LocalModelID:
			dto.Reason = "Select the supported local Parakeet model."
		case !status.Available:
			dto.Reason = "The local speech runtime is unavailable on this platform."
		case !status.Installed:
			dto.Reason = "Download the local speech model to enable dictation."
		case status.Error != "":
			dto.Ready = true
			dto.Reason = "A previous model open failed; the next transcription will retry."
		default:
			dto.Ready = true
		}
	case web.VoiceProviderProvider:
		if loaded.settings.Model == "" {
			dto.Reason = "Select a speech-to-text model."
			break
		}
		if loaded.providerCfg == nil || loaded.apiKey == "" {
			dto.Reason = "Configure a provider endpoint and credentials for this profile."
			break
		}
		models, err := b.transcriptionModels()(ctx, providerConfig(loaded.providerCfg, loaded.apiKey))
		if errors.Is(err, voice.ErrTranscriptionModelsUnsupported) {
			dto.Reason = "This provider cannot verify speech-to-text models."
			break
		}
		if err != nil {
			dto.Reason = "Speech-to-text models are unavailable for this profile."
			break
		}
		if !slices.Contains(models, loaded.settings.Model) {
			dto.Reason = "The selected model is not verified for speech-to-text."
			break
		}
		dto.Ready = true
	default:
		dto.Reason = "The saved voice backend is unsupported."
	}
	return dto, nil
}

func (b *webVoiceBackend) Save(ctx context.Context, slug string, settings store.VoiceSettings, _ uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := b.profileBySlug(ctx, slug)
	if err != nil {
		return err
	}
	db, err := b.openProfileDB(p.Slug)
	if err != nil {
		return errors.New("web: open profile settings failed")
	}
	defer func() { _ = db.Close() }()
	if err := store.NewVoiceSettingsRepo(db).Save(ctx, settings); err != nil {
		return err
	}
	return nil
}

func (b *webVoiceBackend) Models(ctx context.Context, slug string) ([]string, error) {
	loaded, err := b.loadVoiceProfile(ctx, slug)
	if err != nil {
		return nil, err
	}
	if loaded.providerCfg == nil || loaded.apiKey == "" {
		return nil, web.ErrVoiceNotReady
	}
	models, err := b.transcriptionModels()(ctx, providerConfig(loaded.providerCfg, loaded.apiKey))
	if err != nil {
		return nil, err
	}
	return models, nil
}

func (b *webVoiceBackend) Transcribe(ctx context.Context, slug string, wav []byte) (string, error) {
	if err := voice.ValidateWAV(wav); err != nil {
		return "", err
	}
	loaded, err := b.loadVoiceProfile(ctx, slug)
	if err != nil {
		return "", err
	}
	switch loaded.settings.Provider {
	case web.VoiceProviderLocal:
		if loaded.settings.Model != voice.LocalModelID {
			return "", web.ErrVoiceNotReady
		}
		status := b.ModelStatus()
		if !status.Available || !status.Installed {
			return "", voice.ErrLocalModelNotInstalled
		}
		transcriber, err := b.models.LocalTranscriber()
		if err != nil {
			return "", err
		}
		return transcriber.Transcribe(ctx, wav)
	case web.VoiceProviderProvider:
		if loaded.providerCfg == nil || loaded.apiKey == "" || loaded.settings.Model == "" {
			return "", web.ErrVoiceNotReady
		}
		models, err := b.transcriptionModels()(ctx, providerConfig(loaded.providerCfg, loaded.apiKey))
		if err != nil {
			return "", err
		}
		if !slices.Contains(models, loaded.settings.Model) {
			return "", web.ErrVoiceNotReady
		}
		transcriber := b.providerTranscriber()(providerConfig(loaded.providerCfg, loaded.apiKey), loaded.settings.Model)
		if transcriber == nil {
			return "", web.ErrVoiceBackendUnavailable
		}
		return transcriber.Transcribe(ctx, wav)
	default:
		return "", web.ErrVoiceNotReady
	}
}

func (b *webVoiceBackend) ModelStatus() voice.ModelStatus {
	if b.models == nil {
		return voice.ModelStatus{Error: "local speech model manager is unavailable"}
	}
	status := b.models.Status()
	b.mu.Lock()
	operation := b.operation
	b.mu.Unlock()
	switch operation {
	case "download":
		status.Downloading = true
	case "delete":
		status.Deleting = true
	}
	return status
}

func (b *webVoiceBackend) Download(ctx context.Context) error {
	if b.models == nil {
		return web.ErrVoiceBackendUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	status := b.ModelStatus()
	if !status.Available {
		return voice.ErrLocalRuntimeUnavailable
	}
	b.mu.Lock()
	if b.operation != "" || status.Downloading || status.Deleting {
		b.mu.Unlock()
		return web.ErrVoiceOperationConflict
	}
	if status.Installed && status.Verified {
		b.mu.Unlock()
		return nil
	}
	serviceCtx := b.serviceCtx
	if serviceCtx == nil {
		serviceCtx = context.Background()
	}
	if serviceCtx.Err() != nil {
		b.mu.Unlock()
		return web.ErrServiceStopped
	}
	done := make(chan struct{})
	b.operation = "download"
	b.operationDone = done
	b.mu.Unlock()
	go func() {
		_ = b.models.Download(serviceCtx)
		b.finishOperation(done)
	}()
	return nil
}

func (b *webVoiceBackend) Delete(ctx context.Context) error {
	if b.models == nil {
		return web.ErrVoiceBackendUnavailable
	}
	status := b.ModelStatus()
	b.mu.Lock()
	if b.operation != "" || status.Downloading || status.Deleting {
		b.mu.Unlock()
		return web.ErrVoiceOperationConflict
	}
	done := make(chan struct{})
	b.operation = "delete"
	b.operationDone = done
	b.mu.Unlock()
	err := b.models.Delete(ctx)
	b.finishOperation(done)
	return err
}

func (b *webVoiceBackend) Wait(ctx context.Context) error {
	b.mu.Lock()
	done := b.operationDone
	b.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *webVoiceBackend) finishOperation(done chan struct{}) {
	b.mu.Lock()
	if b.operationDone == done {
		b.operation = ""
		b.operationDone = nil
		close(done)
	}
	b.mu.Unlock()
}

type webVoiceProfile struct {
	profile     *store.Profile
	settings    store.VoiceSettings
	providerCfg *store.ProviderConfig
	apiKey      string
}

func (b *webVoiceBackend) loadVoiceProfile(ctx context.Context, slug string) (webVoiceProfile, error) {
	p, err := b.profileBySlug(ctx, slug)
	if err != nil {
		return webVoiceProfile{}, err
	}
	db, err := b.openProfileDB(p.Slug)
	if err != nil {
		return webVoiceProfile{}, errors.New("web: open profile settings failed")
	}
	defer func() { _ = db.Close() }()
	settings, err := store.NewVoiceSettingsRepo(db).Get(ctx)
	if err != nil {
		return webVoiceProfile{}, errors.New("web: read voice settings failed")
	}
	cfg, apiKey, err := loadSettingsProviderConfig(ctx, db, p.ID)
	if err != nil {
		return webVoiceProfile{}, errors.New("web: read provider settings failed")
	}
	return webVoiceProfile{profile: p, settings: settings, providerCfg: cfg, apiKey: apiKey}, nil
}

func (b *webVoiceBackend) profileBySlug(ctx context.Context, slug string) (*store.Profile, error) {
	profilesService := b.profiles
	if profilesService == nil {
		profilesService = profile.NewService(nil)
	}
	profiles, err := profilesService.List(ctx)
	if err != nil {
		return nil, errors.New("web: list profiles failed")
	}
	for _, p := range profiles {
		if p.Slug == slug {
			return p, nil
		}
	}
	return nil, web.ErrSettingsProfileNotFound
}

func (b *webVoiceBackend) openProfileDB(slug string) (*store.DB, error) {
	openDB := b.openDB
	if openDB == nil {
		openDB = openProfileDB
	}
	return openDB(slug)
}

func (b *webVoiceBackend) transcriptionModels() func(context.Context, provider.Config) ([]string, error) {
	if b.listTranscriptionModels != nil {
		return b.listTranscriptionModels
	}
	return voice.ListTranscriptionModels
}

func (b *webVoiceBackend) providerTranscriber() func(provider.Config, string) voice.Transcriber {
	if b.newProviderTranscriber != nil {
		return b.newProviderTranscriber
	}
	return voice.NewProviderTranscriber
}

func providerConfig(cfg *store.ProviderConfig, apiKey string) provider.Config {
	return provider.Config{ProviderType: cfg.ProviderType, Endpoint: cfg.Endpoint, APIKey: apiKey}
}
