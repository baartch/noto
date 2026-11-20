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
	profiles            *profile.Service
	openDB              func(string) (*store.DB, error)
	writeActive         func(string, time.Time) error
	listModels          func(context.Context, provider.Config) ([]provider.ModelInfo, error)
	listEmbeddingModels func(context.Context, provider.Config) ([]provider.ModelInfo, error)
	persistModel        func(context.Context, *store.DB, string, string) error
	persistModels       func(context.Context, *store.DB, string, string, string, string) error
	newSession          func(context.Context, *store.DB, *store.Profile, string, *store.ProviderConfig, string) (web.ChatSession, error)
	mu                  sync.Mutex
	recoveryRequired    bool
}

func newWebSettingsBackend() *webSettingsBackend {
	return &webSettingsBackend{
		profiles:            profile.NewService(nil),
		openDB:              openProfileDB,
		writeActive:         config.WriteActiveProfile,
		listModels:          provider.ListModels,
		listEmbeddingModels: provider.ListEmbeddingModels,
		persistModel:        setActiveModelTx,
		persistModels:       setProviderModelsTx,
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
	result := web.ProfileModelsDTO{ProfileSlug: selected.Slug, Models: []string{}, EmbeddingModels: []string{}}
	if providerCfg != nil {
		result.SavedModel = providerCfg.EffectiveModel()
		result.SavedEmbeddingsModel = providerCfg.EmbeddingsModel
		result.SavedExtractorModel = providerCfg.ExtractorModel
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
	embeddingModels, err := b.listEmbeddingModels(ctx, provider.Config{
		ProviderType: providerCfg.ProviderType,
		Endpoint:     providerCfg.Endpoint,
		APIKey:       apiKey,
	})
	if err != nil {
		result.EmbeddingModelsUnavailable = true
		return result, nil
	}
	for _, model := range embeddingModels {
		result.EmbeddingModels = append(result.EmbeddingModels, model.ID)
	}
	return result, nil
}

func (b *webSettingsBackend) Prepare(ctx context.Context, current web.ProfileRuntime, slug, model string) (web.PreparedSettings, error) {
	return b.PrepareModels(ctx, current, slug, web.ModelSettingsSelection{Model: model})
}

func (b *webSettingsBackend) PrepareModels(ctx context.Context, current web.ProfileRuntime, slug string, selection web.ModelSettingsSelection) (web.PreparedSettings, error) {
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
	selectedModel := selection.Model
	selectedEmbeddingsModel := ""
	selectedExtractorModel := ""
	if providerCfg != nil {
		selectedEmbeddingsModel = providerCfg.EmbeddingsModel
		selectedExtractorModel = providerCfg.ExtractorModel
	}
	if selection.EmbeddingsModel != nil {
		selectedEmbeddingsModel = *selection.EmbeddingsModel
	}
	if selection.ExtractorModel != nil {
		selectedExtractorModel = *selection.ExtractorModel
	}
	var modelInfos []provider.ModelInfo
	var embeddingModelInfos []provider.ModelInfo
	embeddingModelsUnavailable := false
	switch {
	case providerCfg == nil:
		if selectedModel != "" || selectedEmbeddingsModel != "" || selectedExtractorModel != "" {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelNotFound
		}
		selectedModel = ""
	case apiKey == "":
		if (selectedModel != "" && selectedModel != providerCfg.EffectiveModel()) ||
			selectedEmbeddingsModel != providerCfg.EmbeddingsModel || selectedExtractorModel != providerCfg.ExtractorModel {
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
		embeddingModelInfos, err = b.listEmbeddingModels(ctx, provider.Config{ProviderType: providerCfg.ProviderType, Endpoint: providerCfg.Endpoint, APIKey: apiKey})
		if err != nil {
			embeddingModelsUnavailable = true
			// Clearing embeddings is a supported way to disable vector retrieval;
			// it does not select an unavailable provider model.
			if selectedEmbeddingsModel != providerCfg.EmbeddingsModel && selectedEmbeddingsModel != "" {
				closeCandidate(nil)
				return nil, web.ErrSettingsModelListUnavailable
			}
		}
		if selectedModel == "" {
			selectedModel = providerCfg.EffectiveModel()
		}
		if selectedModel == "" || (selectedModel != providerCfg.EffectiveModel() && !containsModel(modelInfos, selectedModel)) {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelNotFound
		}
		if !embeddingModelsUnavailable && selectedEmbeddingsModel != "" && selectedEmbeddingsModel != providerCfg.EmbeddingsModel && !containsModel(embeddingModelInfos, selectedEmbeddingsModel) {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelNotFound
		}
		// Extractor models are completion models, so validate them against /models.
		if selectedExtractorModel != "" && selectedExtractorModel != providerCfg.ExtractorModel && !containsModel(modelInfos, selectedExtractorModel) {
			closeCandidate(nil)
			return nil, web.ErrSettingsModelNotFound
		}
		setupRequired = false
	}

	previousActiveModel := ""
	previousEmbeddingsModel := ""
	previousExtractorModel := ""
	if providerCfg != nil {
		previousActiveModel = providerCfg.ActiveModel
		previousEmbeddingsModel = providerCfg.EmbeddingsModel
		previousExtractorModel = providerCfg.ExtractorModel
	}
	chatModelChanged := providerCfg != nil && apiKey != "" && selectedModel != providerCfg.EffectiveModel()
	embeddingsModelChanged := providerCfg != nil && apiKey != "" && selectedEmbeddingsModel != providerCfg.EmbeddingsModel
	extractorModelChanged := providerCfg != nil && apiKey != "" && selectedExtractorModel != providerCfg.ExtractorModel
	modelChanged := chatModelChanged || embeddingsModelChanged || extractorModelChanged
	if err := ctx.Err(); err != nil {
		closeCandidate(nil)
		return nil, err
	}
	if sameProfile && chatModelChanged && current.Session != nil {
		if _, ok := current.Session.(interface{ SetModel(string) }); !ok {
			return nil, errors.New("web: active session cannot change models")
		}
	}
	if sameProfile && embeddingsModelChanged && current.Session != nil {
		if _, ok := current.Session.(interface {
			ChangeEmbeddingModel(context.Context, string) error
		}); !ok {
			return nil, errors.New("web: active session cannot change embeddings models")
		}
	}
	if sameProfile && extractorModelChanged && current.Session != nil {
		if _, ok := current.Session.(interface{ SetExtractorAdapter(provider.Adapter) }); !ok {
			return nil, errors.New("web: active session cannot change extractor models")
		}
	}
	var candidateSession web.ChatSession
	if !sameProfile && !setupRequired {
		cfgCopy := *providerCfg
		cfgCopy.ActiveModel = selectedModel
		// The candidate session must start from the currently persisted embedding
		// space; Commit migrates it only after the profile marker is written.
		cfgCopy.EmbeddingsModel = providerCfg.EmbeddingsModel
		cfgCopy.ExtractorModel = selectedExtractorModel
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
			extractorModel := selectedExtractorModel
			extractorFallback := extractorModel == ""
			if extractorFallback {
				extractorModel = selectedModel
			}
			applySessionModelMetadata(session, modelInfos, selectedModel, extractorModel, extractorFallback)
		}
		if embeddingsModelChanged {
			if _, ok := candidateSession.(interface {
				ChangeEmbeddingModel(context.Context, string) error
			}); !ok {
				closeCandidate(candidateSession)
				return nil, errors.New("web: candidate session cannot change embeddings models")
			}
		}
	}
	return &webSettingsPreparation{
		backend: b, current: current, selected: selected, db: db, ownsDB: ownsDB,
		providerCfg: providerCfg, modelInfos: modelInfos, embeddingModelInfos: embeddingModelInfos, apiKey: apiKey,
		selectedModel: selectedModel, selectedEmbeddingsModel: selectedEmbeddingsModel, selectedExtractorModel: selectedExtractorModel,
		setupRequired:       setupRequired,
		previousActiveModel: previousActiveModel, previousEmbeddingsModel: previousEmbeddingsModel, previousExtractorModel: previousExtractorModel,
		modelChanged: modelChanged, chatModelChanged: chatModelChanged, embeddingsModelChanged: embeddingsModelChanged, extractorModelChanged: extractorModelChanged,
		candidateSession: candidateSession,
	}, nil
}

type webSettingsPreparation struct {
	backend                 *webSettingsBackend
	current                 web.ProfileRuntime
	selected                *store.Profile
	db                      *store.DB
	ownsDB                  bool
	providerCfg             *store.ProviderConfig
	modelInfos              []provider.ModelInfo
	embeddingModelInfos     []provider.ModelInfo
	apiKey                  string
	selectedModel           string
	selectedEmbeddingsModel string
	selectedExtractorModel  string
	setupRequired           bool
	previousActiveModel     string
	previousEmbeddingsModel string
	previousExtractorModel  string
	modelChanged            bool
	chatModelChanged        bool
	embeddingsModelChanged  bool
	extractorModelChanged   bool
	candidateSession        web.ChatSession
	finished                bool
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
	persist := func(ctx context.Context, activeModel, embeddingsModel, extractorModel string) error {
		if p.embeddingsModelChanged || p.extractorModelChanged {
			return p.backend.persistModels(ctx, p.db, p.selected.ID, activeModel, embeddingsModel, extractorModel)
		}
		return p.backend.persistModel(ctx, p.db, p.selected.ID, activeModel)
	}
	rollback := func() error {
		return persist(context.Background(), p.previousActiveModel, p.previousEmbeddingsModel, p.previousExtractorModel)
	}
	rollbackProfileAndModels := func() error {
		var markerErr, modelsErr error
		markerErr = p.rollbackProfileMarker()
		if p.modelChanged {
			modelsErr = rollback()
		}
		return errors.Join(markerErr, modelsErr)
	}
	if p.modelChanged {
		if err := persist(ctx, p.selectedModel, p.selectedEmbeddingsModel, p.selectedExtractorModel); err != nil {
			return web.ProfileRuntime{}, errors.New("web: model settings could not be saved")
		}
	}
	if err := ctx.Err(); err != nil {
		if p.modelChanged && rollback() != nil {
			p.backend.markRecoveryRequired()
			return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
		}
		return web.ProfileRuntime{}, err
	}
	if p.current.Profile == nil || p.current.Profile.Slug != p.selected.Slug {
		if err := p.backend.writeActive(p.selected.Slug, time.Now().UTC()); err != nil {
			if p.modelChanged && rollback() != nil {
				p.backend.markRecoveryRequired()
				return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
			}
			return web.ProfileRuntime{}, errors.New("web: profile could not be activated")
		}
		if err := ctx.Err(); err != nil {
			if rollbackProfileAndModels() != nil {
				p.backend.markRecoveryRequired()
				return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
			}
			return web.ProfileRuntime{}, err
		}
		if p.embeddingsModelChanged && p.candidateSession != nil {
			changer := p.candidateSession.(interface {
				ChangeEmbeddingModel(context.Context, string) error
			})
			if err := changer.ChangeEmbeddingModel(ctx, p.selectedEmbeddingsModel); err != nil {
				if rollbackProfileAndModels() != nil {
					p.backend.markRecoveryRequired()
					return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
				}
				return web.ProfileRuntime{}, err
			}
		}
		p.finished = true
		p.ownsDB = false
		return web.NewProfileRuntime(p.db, p.selected, p.candidateSession, p.selectedModel, p.setupRequired), nil
	}
	if p.embeddingsModelChanged && p.current.Session != nil {
		changer := p.current.Session.(interface {
			ChangeEmbeddingModel(context.Context, string) error
		})
		if err := changer.ChangeEmbeddingModel(ctx, p.selectedEmbeddingsModel); err != nil {
			if p.modelChanged && rollback() != nil {
				p.backend.markRecoveryRequired()
				return web.ProfileRuntime{}, web.ErrSettingsRecoveryRequired
			}
			return web.ProfileRuntime{}, err
		}
	}
	if p.modelChanged && p.current.Session != nil {
		if p.chatModelChanged {
			setter := p.current.Session.(interface{ SetModel(string) })
			setter.SetModel(p.selectedModel)
		}
		if p.extractorModelChanged {
			if session, ok := p.current.Session.(interface{ SetExtractorAdapter(provider.Adapter) }); ok {
				session.SetExtractorAdapter(p.selectedExtractorAdapter())
			}
		}
		if session, ok := p.current.Session.(*chatpkg.Session); ok {
			extractorModel := p.selectedExtractorModel
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

func (p *webSettingsPreparation) rollbackProfileMarker() error {
	if p.current.Profile == nil {
		return nil
	}
	return p.backend.writeActive(p.current.Profile.Slug, time.Now().UTC())
}

func (p *webSettingsPreparation) selectedExtractorAdapter() provider.Adapter {
	if p.selectedExtractorModel == "" {
		return nil
	}
	return provider.NewOpenAICompatible(provider.Config{
		ProviderType: p.providerCfg.ProviderType,
		Endpoint:     p.providerCfg.Endpoint,
		Model:        p.selectedExtractorModel,
		APIKey:       p.apiKey,
	})
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

func setProviderModelsTx(ctx context.Context, db *store.DB, profileID, chatModel, embeddingsModel, extractorModel string) error {
	return db.WithTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE provider_config
			SET active_model = ?, embeddings_model = ?, extractor_model = ?, updated_at = datetime('now')
			WHERE profile_id = ? AND is_active = 1
		`, chatModel, embeddingsModel, extractorModel, profileID)
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
