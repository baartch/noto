package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	chatpkg "noto/internal/chat"
	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
)

var errProviderSetupRequired = errors.New("app: provider setup required")

type sessionCallbacks struct {
	OnNotes         chatpkg.NotesCallback
	OnNotesSaving   chatpkg.NotesSavingCallback
	OnContextStatus chatpkg.ContextStatusCallback
	OnStats         func(provider.Stats)
}

// newConfiguredSession wires the shared profile-scoped chat session. Callers
// resolve prompts and provider credentials so they can preserve their own
// setup behavior; the decrypted key is consumed here and never returned.
func newConfiguredSession(
	ctx context.Context,
	db *store.DB,
	profile *store.Profile,
	systemPrompt string,
	providerCfg *store.ProviderConfig,
	apiKey string,
	callbacks sessionCallbacks,
) (*chatpkg.Session, error) {
	if providerCfg == nil || strings.TrimSpace(apiKey) == "" || strings.TrimSpace(providerCfg.EffectiveModel()) == "" {
		return nil, errProviderSetupRequired
	}
	if db == nil || profile == nil || profile.ID == "" || profile.Slug == "" {
		return nil, errors.New("app: profile database and identity are required")
	}

	activeModel := providerCfg.EffectiveModel()
	extractorModel := providerCfg.ExtractorModel
	extractorFallback := extractorModel == ""
	adapterCfg := provider.Config{
		ProviderType: "openai_compatible",
		Endpoint:     providerCfg.Endpoint,
		APIKey:       apiKey,
	}

	var extractorAdapter provider.Adapter
	if !extractorFallback {
		extractorAdapter = provider.NewOpenAICompatible(provider.Config{
			ProviderType: "openai_compatible",
			Endpoint:     adapterCfg.Endpoint,
			Model:        extractorModel,
			APIKey:       adapterCfg.APIKey,
		})
	}

	logger, _ := observe.NewFileLogger("")
	session, err := chatpkg.NewSession(
		ctx,
		profile.ID,
		profile.Slug,
		systemPrompt,
		db,
		store.NewConversationRepo(db),
		store.NewMessageRepo(db),
		store.NewMemoryNoteRepo(db),
		provider.NewOpenAICompatible(provider.Config{
			ProviderType: "openai_compatible",
			Endpoint:     adapterCfg.Endpoint,
			Model:        activeModel,
			APIKey:       adapterCfg.APIKey,
		}),
		extractorAdapter,
		logger,
		callbacks.OnNotes,
		callbacks.OnNotesSaving,
		callbacks.OnContextStatus,
		callbacks.OnStats,
	)
	if err != nil {
		return nil, fmt.Errorf("app: create chat session: %w", err)
	}
	return session, nil
}
