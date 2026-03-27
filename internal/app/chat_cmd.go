package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"noto/internal/chat"
	"noto/internal/commands"
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

	// Build command registry + dispatcher.
	registry := commands.NewRegistry()
	if err := commands.RegisterProfileCommands(registry); err != nil {
		return err
	}
	if err := commands.RegisterPromptCommands(registry); err != nil {
		return err
	}

	dispatcher := chat.NewDispatcher(registry)
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

	// Build TUI callbacks.
	var providerFn tui.ProviderFunc
	var listModelsFn tui.ListModelsFunc
	activeModel := ""

	if providerCfg != nil && decryptedKey != "" {
		activeModel = providerCfg.EffectiveModel()

		adapterCfg := provider.Config{
			ProviderType: "openai_compatible",
			Endpoint:     providerCfg.Endpoint,
			Model:        activeModel,
			APIKey:       decryptedKey,
		}

		providerFn = func(ctx context.Context, userMsg string) (string, error) {
			adapter := provider.NewOpenAICompatible(provider.Config{
				ProviderType: adapterCfg.ProviderType,
				Endpoint:     adapterCfg.Endpoint,
				Model:        activeModel, // captured by reference so /model updates take effect
				APIKey:       adapterCfg.APIKey,
			})
			resp, err := adapter.Complete(ctx, provider.CompletionRequest{
				Messages:    []provider.Message{{Role: "user", Content: userMsg}},
				Model:       activeModel,
				Temperature: 0.7,
			})
			if err != nil {
				return "", err
			}
			return resp.Content, nil
		}

		listModelsFn = func(ctx context.Context) ([]provider.ModelInfo, error) {
			return provider.ListModels(ctx, adapterCfg)
		}
	}

	// modelSelected persists the chosen model to the DB and updates activeModel.
	cfgRepo := store.NewProviderConfigRepo(db)
	modelSelectedFn := func(modelID string) error {
		if err := cfgRepo.SetModel(ctx, activeProfile.ID, modelID); err != nil {
			return err
		}
		activeModel = modelID // update closure so next provider call uses new model
		return nil
	}

	m := tui.New(
		activeProfile.Name,
		activeModel,
		dispatcher,
		execCtx,
		providerFn,
		listModelsFn,
		modelSelectedFn,
	)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("chat: TUI error: %w", err)
	}
	return nil
}

// loadProviderConfig reads the active provider config and decrypts the API key.
// Returns (nil, "") if nothing is configured.
func loadProviderConfig(ctx context.Context, db *store.DB, profileID string) (*store.ProviderConfig, string) {
	// Env vars take priority.
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
