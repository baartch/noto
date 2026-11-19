package app

import (
	"context"
	"errors"
	"os"

	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/security"
	"noto/internal/store"
	"noto/internal/web"
)

type webSettingsBackend struct {
	profiles *profile.Service
}

func newWebSettingsBackend() *webSettingsBackend {
	return &webSettingsBackend{profiles: profile.NewService(nil)}
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

	db, err := openProfileDB(selected.Slug)
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
	models, err := provider.ListModels(ctx, provider.Config{
		ProviderType: "openai_compatible",
		Endpoint:     providerCfg.Endpoint,
		APIKey:       apiKey,
	})
	if err != nil {
		return web.ProfileModelsDTO{}, errors.New("web: provider model lookup failed")
	}
	result.Models = make([]string, 0, len(models))
	for _, model := range models {
		result.Models = append(result.Models, model.ID)
	}
	return result, nil
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
