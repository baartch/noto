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

	db, err := openGlobalDB()
	if err != nil {
		return fmt.Errorf("chat: open db: %w", err)
	}
	defer db.Close()

	// Resolve or auto-create active profile.
	profRepo := store.NewProfileRepo(db)
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

	// Build command registry + slash dispatcher.
	registry := commands.NewRegistry()
	if err := commands.RegisterProfileCommands(registry); err != nil {
		return err
	}
	if err := commands.RegisterPromptCommands(registry); err != nil {
		return err
	}
	dispatcher := chatpkg.NewDispatcher(registry)
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
	}

	// Resolve provider config.
	providerCfg, decryptedKey := loadProviderConfig(ctx, db, activeProfile.ID)
	activeModel := ""

	var providerFn tui.ProviderFunc
	var listModelsFn tui.ListModelsFunc

	if providerCfg != nil && decryptedKey != "" {
		activeModel = providerCfg.EffectiveModel()

		adapterCfg := provider.Config{
			ProviderType: "openai_compatible",
			Endpoint:     providerCfg.Endpoint,
			APIKey:       decryptedKey,
		}

		// We need a live reference to the tea.Program so extraction callbacks
		// can send messages into it. We create a pointer-to-pointer and fill it
		// after p := tea.NewProgram(...).
		var prog **tea.Program
		progHolder := new(*tea.Program)
		prog = progHolder

		// Load the profile system prompt.
		systemPrompt := loadSystemPrompt(activeProfile.Slug)

		// Repositories for the session.
		convRepo    := store.NewConversationRepo(db)
		msgRepo     := store.NewMessageRepo(db)
		noteRepo    := store.NewMemoryNoteRepo(db)
		summaryRepo := store.NewSessionSummaryRepo(db)
		logger      := observe.NewNoopLogger()

		// Create the session — this assembles the system prompt with memory notes.
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
				// Called from background goroutine — send into TUI via program.
				if *prog != nil {
					(*prog).Send(tui.NotesSaved(count))
				}
			},
		)
		if sessErr != nil {
			return fmt.Errorf("chat: start session: %w", sessErr)
		}
		defer sess.Close(context.Background())

		providerFn = func(callCtx context.Context, userMsg string) (string, error) {
			// Update model on the adapter each call so /model changes take effect.
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

		// modelSelected persists the choice and updates the closure variable.
		cfgRepo := store.NewProviderConfigRepo(db)
		modelSelectedFn := func(modelID string) error {
			if err := cfgRepo.SetModel(ctx, activeProfile.ID, modelID); err != nil {
				return err
			}
			activeModel = modelID
			return nil
		}

		m := tui.New(
			activeProfile.Name, activeModel,
			dispatcher, execCtx,
			providerFn, listModelsFn, modelSelectedFn,
		)
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		*prog = p
		if _, runErr := p.Run(); runErr != nil {
			return fmt.Errorf("chat: TUI error: %w", runErr)
		}
		return nil
	}

	// No provider configured — still open TUI but show guidance.
	m := tui.New(
		activeProfile.Name, "",
		dispatcher, execCtx,
		nil, nil,
		func(modelID string) error { return nil },
	)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, runErr := p.Run(); runErr != nil {
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
