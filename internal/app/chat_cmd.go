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

	// Open global DB and resolve active profile.
	db, err := openGlobalDB()
	if err != nil {
		return fmt.Errorf("chat: open db: %w", err)
	}
	defer db.Close()

	profRepo := store.NewProfileRepo(db)
	profSvc := profile.NewService(profRepo)

	activeProfile, err := profSvc.GetActive(ctx)
	if err != nil {
		// No active profile — try the startup flow automatically.
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
			return fmt.Errorf("multiple profiles exist but none is active. Run: noto profile select <name>")
		}
	}

	// Build command registry.
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
			// In TUI mode confirmation is handled by the model; fall back to true for
			// non-destructive slash commands. Destructive commands should prompt inline.
			fmt.Fprintf(os.Stderr, "%s [yes/no]: ", prompt)
			var ans string
			fmt.Scanln(&ans)
			return strings.ToLower(strings.TrimSpace(ans)) == "yes"
		},
	}

	// Resolve provider from environment (NOTO_API_KEY, NOTO_ENDPOINT, NOTO_MODEL)
	// or from DB config. A nil providerFn means "no provider configured".
	providerFn := resolveProvider(ctx, db, activeProfile.ID)

	model := tui.New(activeProfile.Name, dispatcher, execCtx, providerFn)

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("chat: TUI error: %w", err)
	}
	return nil
}

// resolveProvider builds a ProviderFunc from env vars or the stored provider config.
// Returns nil if no provider is configured.
func resolveProvider(ctx context.Context, db *store.DB, profileID string) tui.ProviderFunc {
	// Prefer environment variables.
	apiKey := os.Getenv("NOTO_API_KEY")
	model := os.Getenv("NOTO_MODEL")
	endpoint := os.Getenv("NOTO_ENDPOINT")

	if apiKey == "" {
		// Try reading from DB.
		cfgRepo := store.NewProviderConfigRepo(db)
		cfg, err := cfgRepo.GetActive(ctx, profileID)
		if err != nil {
			if errors.Is(err, store.ErrProviderConfigNotFound) {
				return nil // no provider configured
			}
			return nil
		}
		apiKey = cfg.CredentialRef // already decrypted at this layer (simplification)
		model = cfg.Model
		endpoint = cfg.Endpoint
	}

	if apiKey == "" {
		return nil
	}

	if model == "" {
		model = "gpt-4o-mini"
	}

	adapter := provider.NewOpenAICompatible(provider.Config{
		ProviderType: "openai_compatible",
		Endpoint:     endpoint,
		Model:        model,
		APIKey:       apiKey,
	})

	return func(ctx context.Context, userMsg string) (string, error) {
		resp, err := adapter.Complete(ctx, provider.CompletionRequest{
			Messages:    []provider.Message{{Role: "user", Content: userMsg}},
			Model:       model,
			Temperature: 0.7,
		})
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}
}
