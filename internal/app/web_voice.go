package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

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
	modelCache              *transcriptionModelCache
	modelCacheMu            sync.Mutex

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
		modelCache:              newTranscriptionModelCache(),
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
		models, err := b.cachedTranscriptionModels(ctx, providerConfig(loaded.providerCfg, loaded.apiKey))
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
	if err := validateVoiceSelection(settings); err != nil {
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
	if settings.Provider == web.VoiceProviderProvider {
		if err := b.verifyProviderVoiceModel(ctx, db, p.ID, settings.Model); err != nil {
			return err
		}
	}
	return store.NewVoiceSettingsRepo(db).Save(ctx, settings)
}

func validateVoiceSelection(settings store.VoiceSettings) error {
	switch settings.Provider {
	case "":
		if settings.Model != "" {
			return web.ErrInvalidVoiceSettings
		}
	case web.VoiceProviderLocal:
		if settings.Model != voice.LocalModelID {
			return web.ErrInvalidVoiceSettings
		}
	case web.VoiceProviderProvider:
		if settings.Model == "" || strings.TrimSpace(settings.Model) != settings.Model {
			return web.ErrInvalidVoiceSettings
		}
	default:
		return store.ErrInvalidVoiceProvider
	}
	return nil
}

func (b *webVoiceBackend) verifyProviderVoiceModel(ctx context.Context, db *store.DB, profileID, model string) error {
	cfg, apiKey, err := loadSettingsProviderConfig(ctx, db, profileID)
	if err != nil {
		return errors.New("web: read provider settings failed")
	}
	if cfg == nil || apiKey == "" {
		return web.ErrInvalidVoiceSettings
	}
	models, err := b.cachedTranscriptionModels(ctx, providerConfig(cfg, apiKey))
	if err != nil {
		return err
	}
	if !slices.Contains(models, model) {
		return web.ErrVoiceModelUnverified
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
	models, err := b.cachedTranscriptionModels(ctx, providerConfig(loaded.providerCfg, loaded.apiKey))
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
		models, err := b.cachedTranscriptionModels(ctx, providerConfig(loaded.providerCfg, loaded.apiKey))
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

const transcriptionModelCacheTTL = time.Minute

type transcriptionModelCacheKey struct {
	endpoint string
	apiKey   [32]byte
}

type transcriptionModelCacheEntry struct {
	models    []string
	expiresAt time.Time
}

type transcriptionModelCache struct {
	mu      sync.Mutex
	entries map[transcriptionModelCacheKey]transcriptionModelCacheEntry
	ttl     time.Duration
	now     func() time.Time
}

func newTranscriptionModelCache() *transcriptionModelCache {
	return &transcriptionModelCache{
		entries: make(map[transcriptionModelCacheKey]transcriptionModelCacheEntry),
		ttl:     transcriptionModelCacheTTL,
		now:     time.Now,
	}
}

func (c *transcriptionModelCache) get(ctx context.Context, cfg provider.Config, fetch func(context.Context, provider.Config) ([]string, error)) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := transcriptionModelCacheKey{endpoint: cfg.Endpoint, apiKey: sha256.Sum256([]byte(cfg.APIKey))}
	now := c.now()
	c.mu.Lock()
	for cachedKey, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, cachedKey)
		}
	}
	if entry, ok := c.entries[key]; ok {
		if now.Before(entry.expiresAt) {
			models := slices.Clone(entry.models)
			c.mu.Unlock()
			return models, nil
		}
		delete(c.entries, key)
	}
	c.mu.Unlock()

	models, err := fetch(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[transcriptionModelCacheKey]transcriptionModelCacheEntry)
	}
	c.entries[key] = transcriptionModelCacheEntry{models: slices.Clone(models), expiresAt: c.now().Add(c.ttl)}
	c.mu.Unlock()
	return slices.Clone(models), nil
}

func (b *webVoiceBackend) cachedTranscriptionModels(ctx context.Context, cfg provider.Config) ([]string, error) {
	b.modelCacheMu.Lock()
	if b.modelCache == nil {
		b.modelCache = newTranscriptionModelCache()
	}
	cache := b.modelCache
	b.modelCacheMu.Unlock()
	return cache.get(ctx, cfg, b.transcriptionModels())
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
