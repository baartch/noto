package app

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"time"

	chatpkg "noto/internal/chat"
	"noto/internal/config"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/security"
	"noto/internal/store"
	"noto/internal/web"
)

type webSettingsBackend struct {
	profiles         *profile.Service
	openDB           func(string) (*store.DB, error)
	writeActive      func(string, time.Time) error
	listModels       func(context.Context, provider.Config) ([]provider.ModelInfo, error)
	persistModel     func(context.Context, *store.DB, string, string) error
	newSession       func(context.Context, *store.DB, *store.Profile, string, *store.ProviderConfig, string) (web.ChatSession, error)
	mu               sync.Mutex
	recoveryRequired bool
}

func newWebSettingsBackend() *webSettingsBackend {
	return &webSettingsBackend{
		profiles:     profile.NewService(nil),
		openDB:       openProfileDB,
		writeActive:  config.WriteActiveProfile,
		listModels:   provider.ListModels,
		persistModel: setActiveModelTx,
		newSession: func(ctx context.Context, db *store.DB, p *store.Profile, prompt string, cfg *store.ProviderConfig, key string) (web.ChatSession, error) {
			return newConfiguredSession(ctx, db, p, prompt, cfg, key, sessionCallbacks{})
		},
	}
}

func (b *webSettingsBackend) Profiles(ctx context.Context) ([]web.SettingsProfileDTO, error) {
	profiles, err := b.profiles.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]web.SettingsProfileDTO, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, web.SettingsProfileDTO{Slug: p.Slug, Name: p.Name})
	}
	return out, nil
}

func (b *webSettingsBackend) Models(ctx context.Context, slug string) (web.ProfileModelsDTO, error) {
	profiles, err := b.profiles.List(ctx)
	if err != nil {
		return web.ProfileModelsDTO{}, err
	}
	var selected *store.Profile
	for _, p := range profiles {
		if p.Slug == slug {
			selected = p
			break
		}
	}
	if selected == nil {
		return web.ProfileModelsDTO{}, web.ErrSettingsProfileNotFound
	}

	db, err := b.openDB(selected.Slug)
	if err != nil {
		return web.ProfileModelsDTO{}, err
	}
	defer func() { _ = db.Close() }()

	providerCfg, apiKey, err := loadSettingsProviderConfig(ctx, db, selected.ID)
	if err != nil {
		return web.ProfileModelsDTO{}, err
	}
	result := web.ProfileModelsDTO{ProfileSlug: selected.Slug, Models: []string{}}
	if providerCfg != nil {
		result.SavedModel = providerCfg.EffectiveModel()
	}
	if providerCfg == nil || apiKey == "" || result.SavedModel == "" {
		result.SetupRequired = true
		return result, nil
	}
	models, err := b.listModels(ctx, provider.Config{
		ProviderType: providerCfg.ProviderType,
		Endpoint:     providerCfg.Endpoint,
		APIKey:       apiKey,
	})
	if err != nil {
		return web.ProfileModelsDTO{}, web.ErrSettingsModelListUnavailable
	}
	result.Models = make([]string, 0, len(models))
	for _, model := range models {
		result.Models = append(result.Models, model.ID)
	}
	return result, nil
}

func (b *webSettingsBackend) Prepare(ctx context.Context, current web.ProfileRuntime, slug, model string) (web.PreparedSettings, error) {
	b.mu.Lock()
	if b.recoveryRequired {
		b.mu.Unlock()
		return nil, web.ErrSettingsRecoveryRequired
	}
	b.mu.Unlock()

	profiles, err := b.profiles.List(ctx)
	if err != nil {
		return nil, errors.New("web: profile list is unavailable")
	}
	var selected *store.Profile
	for _, p := range profiles {
		if p.Slug == slug {
			selected = p
			break
		}
	}
	if selected == nil {
		return nil, web.ErrSettingsProfileNotFound
	}
	sameProfile := current.Profile != nil && current.Profile.Slug == selected.Slug
	db := current.DB
	ownsDB := false
	if !sameProfile {
		db, err = b.openDB(selected.Slug)
		if err != nil {
			return nil, errors.New("web: profile database is unavailable")
		}
		ownsDB = true
	}
	closeCandidate := func(session web.ChatSession) {
		if session != nil {
			if closer, ok := session.(interface{ Close(context.Context) }); ok {
				closer.Close(context.Background())
			}
		}
		if ownsDB && db != nil {
			_ = db.Close()
		}
	}

	providerCfg, apiKey, err := loadSettingsProviderConfig(ctx, db, selected.ID)
	if err != nil {
		closeCandidate(nil)
		return nil, err
	}
	setupRequired := providerCfg == nil || apiKey == "" || providerCfg.EffectiveModel() == ""
	selectedModel := model
	var modelInfos []provider.ModelInfo
	switch {
	case providerCfg == nil:
		if model != "" {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelNotFound
		}
		selectedModel = ""
	case apiKey == "":
		if model != "" && model != providerCfg.EffectiveModel() {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelNotFound
		}
		selectedModel = providerCfg.EffectiveModel()
		setupRequired = true
	default:
		modelInfos, err = b.listModels(ctx, provider.Config{ProviderType: providerCfg.ProviderType, Endpoint: providerCfg.Endpoint, APIKey: apiKey})
		if err != nil {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelListUnavailable
		}
		if selectedModel == "" {
			selectedModel = providerCfg.EffectiveModel()
		}
		if selectedModel == "" || (selectedModel != providerCfg.EffectiveModel() && !containsModel(modelInfos, selectedModel)) {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelNotFound
		}
		setupRequired = false
	}

	previousActiveModel := ""
	if providerCfg != nil {
		previousActiveModel = providerCfg.ActiveModel
	}
	modelChanged := providerCfg != nil && apiKey != "" && selectedModel != providerCfg.EffectiveModel()
	if err := ctx.Err(); err != nil {
		closeCandidate(nil)
		return nil, err
	}
	if sameProfile && modelChanged && current.Session != nil {
		if _, ok := current.Session.(interface{ SetModel(string) }); !ok {
			return nil, errors.New("web: active session cannot change models")
		}
	}
	var candidateSession web.ChatSession
	if !sameProfile && !setupRequired {
		cfgCopy := *providerCfg
		cfgCopy.ActiveModel = selectedModel
		prompt, _ := loadSystemPrompt(ctx, db, selected)
		candidateSession, err = b.newSession(ctx, db, selected, prompt, &cfgCopy, apiKey)
		if err != nil {
			closeCandidate(candidateSession)
			return nil, errors.New("web: chat session is unavailable")
		}
		if err := ctx.Err(); err != nil {
			closeCandidate(candidateSession)
			return nil, err
		}
		if session, ok := candidateSession.(*chatpkg.Session); ok {
			extractorModel := providerCfg.ExtractorModel
			extractorFallback := extractorModel == ""
			if extractorFallback {
				extractorModel = selectedModel
			}
			applySessionModelMetadata(session, modelInfos, selectedModel, extractorModel, extractorFallback)
		}
	}
	return &webSettingsPreparation{
		backend: b, current: current, selected: selected, db: db, ownsDB: ownsDB,
		providerCfg: providerCfg, modelInfos: modelInfos, apiKey: apiKey,
		selectedModel: selectedModel, setupRequired: setupRequired,
		previousActiveModel: previousActiveModel, modelChanged: modelChanged,
		candidateSession: candidateSession,
	}, nil
}

type webSettingsPreparation struct {
	backend             *webSettingsBackend
	current             web.ProfileRuntime
	selected            *store.Profile
	db                  *store.DB
	ownsDB              bool
	providerCfg         *store.ProviderConfig
	modelInfos          []provider.ModelInfo
	apiKey              string
	selectedModel       string
	setupRequired       bool
	previousActiveModel string
	modelChanged        bool
	candidateSession    web.ChatSession
	finished            bool
}

func (p *webSettingsPreparation) Commit(ctx context.Context) (web.ProfileRuntime, error) {
	if p.finished {
		return web.ProfileRuntime{}, errors.New("web: settings preparation is already finished")
	}
	p.backend.mu.Lock()
	recoveryRequired := p.backend.recoveryRequired
	p.backend.mu.Unlock()
	if recoveryRequired {
		return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
	}
	if err := ctx.Err(); err != nil {
		return web.ProfileRuntime{}, err
	}
	if p.modelChanged {
		if err := p.backend.persistModel(ctx, p.db, p.selected.ID, p.selectedModel); err != nil {
			return web.ProfileRuntime{}, errors.New("web: model could not be saved")
		}
	}
	if err := ctx.Err(); err != nil {
		if p.modelChanged && p.backend.persistModel(context.Background(), p.db, p.selected.ID, p.previousActiveModel) != nil {
			p.backend.markRecoveryRequired()
			return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
		}
		return web.ProfileRuntime{}, err
	}
	if p.current.Profile == nil || p.current.Profile.Slug != p.selected.Slug {
		if err := p.backend.writeActive(p.selected.Slug, time.Now().UTC()); err != nil {
			if p.modelChanged && p.backend.persistModel(context.Background(), p.db, p.selected.ID, p.previousActiveModel) != nil {
				p.backend.markRecoveryRequired()
				return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
			}
			return web.ProfileRuntime{}, errors.New("web: profile could not be activated")
		}
		p.finished = true
		p.ownsDB = false
		return web.NewProfileRuntime(p.db, p.selected, p.candidateSession, p.selectedModel, p.setupRequired), nil
	}
	if p.modelChanged && p.current.Session != nil {
		setter := p.current.Session.(interface{ SetModel(string) })
		setter.SetModel(p.selectedModel)
		if session, ok := p.current.Session.(*chatpkg.Session); ok {
			extractorModel := p.providerCfg.ExtractorModel
			extractorFallback := extractorModel == ""
			if extractorFallback {
				extractorModel = p.selectedModel
			}
			applySessionModelMetadata(session, p.modelInfos, p.selectedModel, extractorModel, extractorFallback)
		}
	}
	p.current.Model = p.selectedModel
	p.current.SetupRequired = p.setupRequired
	p.finished = true
	return p.current, nil
}

func (p *webSettingsPreparation) Abort() {
	if p.finished {
		return
	}
	p.finished = true
	if p.candidateSession != nil {
		if closer, ok := p.candidateSession.(interface{ Close(context.Context) }); ok {
			closer.Close(context.Background())
		}
	}
	if p.ownsDB && p.db != nil {
		_ = p.db.Close()
	}
}

func (b *webSettingsBackend) markRecoveryRequired() {
	b.mu.Lock()
	b.recoveryRequired = true
	b.mu.Unlock()
}

func containsModel(models []provider.ModelInfo, id string) bool {
	for _, model := range models {
		if model.ID == id {
			return true
		}
	}
	return false
}

func setActiveModelTx(ctx context.Context, db *store.DB, profileID, model string) error {
	return db.WithTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE provider_config SET active_model = ?, updated_at = datetime('now') WHERE profile_id = ? AND is_active = 1`, model, profileID)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return store.ErrProviderConfigNotFound
		}
		return nil
	})
}

func loadSettingsProviderConfig(ctx context.Context, db *store.DB, profileID string) (*store.ProviderConfig, string, error) {
	if os.Getenv("NOTO_API_KEY") != "" {
		cfg, apiKey := loadProviderConfig(ctx, db, profileID)
		if cfg == nil {
			return nil, "", errors.New("web: provider configuration is unavailable")
		}
		return cfg, apiKey, nil
	}

	cfg, err := store.NewProviderConfigRepo(db).GetActive(ctx, profileID)
	if errors.Is(err, store.ErrProviderConfigNotFound) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", errors.New("web: provider configuration is unavailable")
	}
	passphrase, err := security.MachinePassphrase()
	if err != nil {
		return cfg, "", nil
	}
	apiKey, err := security.Decrypt(cfg.CredentialRef, passphrase)
	if err != nil {
		return cfg, "", nil
	}
	return cfg, apiKey, nil
}
