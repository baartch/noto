package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	chatpkg "noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/observe"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/security"
	"noto/internal/store"
	"noto/internal/tui"
)

func chatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session with the active profile",
		RunE:  runChat,
	}
}

func runChat(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	// Global DB: profile registry only.
	globalDB, err := openGlobalDB()
	if err != nil {
		return fmt.Errorf("chat: open global db: %w", err)
	}
	defer globalDB.Close()

	// Resolve or auto-create active profile.
	profRepo := store.NewProfileRepo(globalDB)
	profSvc := profile.NewService(profRepo)

	activeProfile, err := profSvc.GetActive(ctx)
	if err != nil {
		profiles, listErr := profSvc.List(ctx)
		if listErr != nil {
			return listErr
		}
		switch len(profiles) {
		case 0:
			fmt.Println("No profiles found. Creating a default profile…")
			p, createErr := profSvc.Create(ctx, "default")
			if createErr != nil {
				return createErr
			}
			if _, selErr := profSvc.Select(ctx, p.Name); selErr != nil {
				return selErr
			}
			activeProfile = p
		case 1:
			if _, selErr := profSvc.Select(ctx, profiles[0].Name); selErr != nil {
				return selErr
			}
			activeProfile = profiles[0]
		default:
			return fmt.Errorf("multiple profiles exist but none is active — run: noto profile select <name>")
		}
	}

	// Single program holder shared by all async callbacks (notes, profile changes).
	// Filled just before p.Run() so closures can reference it safely.
	var prog *tea.Program

	// Build command registry + slash dispatcher.
	registry := commands.NewRegistry()
	if err := commands.RegisterProfileCommands(registry, profSvc); err != nil {
		return err
	}
	if err := commands.RegisterPromptCommands(registry); err != nil {
		return err
	}
	if err := commands.RegisterModelCommand(registry); err != nil {
		return err
	}
	dispatcher := chatpkg.NewDispatcher(registry)

	// Per-profile DB: conversations, messages, memory notes, provider config, etc.
	profileDB, err := openProfileDB(activeProfile.Slug)
	if err != nil {
		return fmt.Errorf("chat: open profile db: %w", err)
	}
	defer profileDB.Close()

	execCtx := &commands.ExecContext{
		ProfileID:   activeProfile.ID,
		ProfileSlug: activeProfile.Slug,
		Output:      os.Stdout,
		Confirm: func(prompt string) bool {
			fmt.Fprintf(os.Stderr, "%s [yes/no]: ", prompt)
			var ans string
			fmt.Scanln(&ans)
			return strings.ToLower(strings.TrimSpace(ans)) == "yes"
		},
		SuspendForEditor: func(fn func() error) error { return fn() },
		OnPromptChanged: func(slug string) error {
			// Prompt edits invalidate the context cache and mark the vector index stale.
			cacheRepo := store.NewContextCacheRepo(profileDB)
			_ = cacheRepo.InvalidateAll(ctx, activeProfile.ID)
			_ = store.NewVectorManifestRepo(profileDB).SetManifestStatusStr(ctx, activeProfile.ID, "stale")
			return nil
		},
	}

	// Resolve provider config.
	providerCfg, decryptedKey := loadProviderConfig(ctx, profileDB, activeProfile.ID)
	activeModel := ""

	var providerFn tui.ProviderFunc
	var listModelsFn tui.ListModelsFunc
	modelSelectedFn := func(modelID string) error { return nil }

	if providerCfg != nil && decryptedKey != "" {
		activeModel = providerCfg.EffectiveModel()

		adapterCfg := provider.Config{
			ProviderType: "openai_compatible",
			Endpoint:     providerCfg.Endpoint,
			APIKey:       decryptedKey,
		}

		systemPrompt := loadSystemPrompt(activeProfile.Slug)

		convRepo    := store.NewConversationRepo(profileDB)
		msgRepo     := store.NewMessageRepo(profileDB)
		noteRepo    := store.NewMemoryNoteRepo(profileDB)
		summaryRepo := store.NewSessionSummaryRepo(profileDB)
		logger      := observe.NewNoopLogger()

		sess, sessErr := chatpkg.NewSession(
			ctx,
			activeProfile.ID,
			systemPrompt,
			convRepo, msgRepo, noteRepo, summaryRepo,
			provider.NewOpenAICompatible(provider.Config{
				ProviderType: "openai_compatible",
				Endpoint:     adapterCfg.Endpoint,
				Model:        activeModel,
				APIKey:       adapterCfg.APIKey,
			}),
			logger,
			func(count int) {
				if prog != nil {
					prog.Send(tui.NotesSaved(count))
				}
			},
		)
		if sessErr != nil {
			return fmt.Errorf("chat: start session: %w", sessErr)
		}
		defer sess.Close(context.Background())

		providerFn = func(callCtx context.Context, userMsg string) (string, error) {
			sess.SetModel(activeModel)
			result, err := sess.Send(callCtx, userMsg)
			if err != nil {
				return "", err
			}
			return result.Reply, nil
		}

		listModelsFn = func(callCtx context.Context) ([]provider.ModelInfo, error) {
			return provider.ListModels(callCtx, adapterCfg)
		}

		cfgRepo := store.NewProviderConfigRepo(profileDB)
		modelSelectedFn = func(modelID string) error {
			if err := cfgRepo.SetModel(ctx, activeProfile.ID, modelID); err != nil {
				return err
			}
			activeModel = modelID
			return nil
		}
	}

	listProfilesFn := func(ctx context.Context) ([]*store.Profile, error) {
		return profSvc.List(ctx)
	}
	// profileSelectedFn just does the DB work and returns the new name.
	// The TUI updates its own state from the return value — no prog.Send().
	profileSelectedFn := func(profileName string) error {
		_, err := profSvc.Select(ctx, profileName)
		return err
	}

	m := tui.New(
		activeProfile.Name, activeModel,
		dispatcher, execCtx,
		providerFn, listModelsFn, modelSelectedFn,
		listProfilesFn, profileSelectedFn,
	)
	prog = tea.NewProgram(m, tea.WithAltScreen())
	if _, runErr := prog.Run(); runErr != nil {
		return fmt.Errorf("chat: TUI error: %w", runErr)
	}
	return nil
}

// loadSystemPrompt reads the profile system prompt file, falling back to a default.
func loadSystemPrompt(slug string) string {
	home, _ := os.UserHomeDir()
	path := home + "/.noto/profiles/" + slug + "/prompts/system.md"
	data, err := os.ReadFile(path)
	if err != nil {
		return "You are a helpful assistant."
	}
	return strings.TrimSpace(string(data))
}

// loadProviderConfig reads the active provider config and decrypts the API key.
func loadProviderConfig(ctx context.Context, db *store.DB, profileID string) (*store.ProviderConfig, string) {
	apiKey := os.Getenv("NOTO_API_KEY")
	if apiKey != "" {
		return &store.ProviderConfig{
			Endpoint:    os.Getenv("NOTO_ENDPOINT"),
			Model:       os.Getenv("NOTO_MODEL"),
			ActiveModel: os.Getenv("NOTO_MODEL"),
		}, apiKey
	}

	cfgRepo := store.NewProviderConfigRepo(db)
	cfg, err := cfgRepo.GetActive(ctx, profileID)
	if err != nil {
		if errors.Is(err, store.ErrProviderConfigNotFound) {
			return nil, ""
		}
		return nil, ""
	}

	passphrase, err := security.MachinePassphrase()
	if err != nil {
		return nil, ""
	}
	decrypted, err := security.Decrypt(cfg.CredentialRef, passphrase)
	if err != nil {
		return nil, ""
	}
	return cfg, decrypted
}
