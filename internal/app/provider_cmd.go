package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"noto/internal/security"
	"noto/internal/store"
)

func providerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage the AI provider configuration for the active profile",
	}
	cmd.AddCommand(providerSetCmd())
	cmd.AddCommand(providerShowCmd())
	cmd.AddCommand(providerClearCmd())
	cmd.AddCommand(providerExtractorModelCmd())
	return cmd
}

func providerSetCmd() *cobra.Command {
	var (
		apiKey         string
		model          string
		extractorModel string
		endpoint       string
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the provider configuration for the active profile",
		Example: `  noto provider set --key sk-...
  noto provider set --endpoint http://localhost:11434/v1/chat/completions --key ollama
  noto provider set --key sk-... --model gpt-4o-mini   # optional default model
  noto provider set --key sk-... --model gpt-4o --extractor-model gpt-4o-mini`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if apiKey == "" {
				return errors.New("--key is required")
			}

			ctx := context.Background()
			globalDB, profileDB, activeProfile, err := openBothDBs(ctx)
			if err != nil {
				return err
			}
			defer func() {
				_ = globalDB.Close()
			}()
			defer func() {
				_ = profileDB.Close()
			}()

			passphrase, err := security.MachinePassphrase()
			if err != nil {
				return fmt.Errorf("provider: get passphrase: %w", err)
			}
			encrypted, err := security.Encrypt(apiKey, passphrase)
			if err != nil {
				return fmt.Errorf("provider: encrypt key: %w", err)
			}

			if err := deactivateProviderConfigs(ctx, profileDB, activeProfile.ID); err != nil {
				return err
			}

			cfg := &store.ProviderConfig{
				ID:             fmt.Sprintf("pc-%x", time.Now().UnixNano()),
				ProfileID:      activeProfile.ID,
				ProviderType:   "openai_compatible",
				Endpoint:       endpoint,
				Model:          model,
				ExtractorModel: extractorModel,
				CredentialRef:  encrypted,
				IsActive:       true,
			}

			repo := store.NewProviderConfigRepo(profileDB)
			if err := repo.Upsert(ctx, cfg); err != nil {
				return err
			}

			fmt.Printf("Provider configured for profile %q\n", activeProfile.Name)
			if model != "" {
				fmt.Printf("  Model:          %s\n", model)
			} else {
				fmt.Printf("  Model:          (none — use /model in chat to select)\n")
			}
			if extractorModel != "" {
				fmt.Printf("  Extractor:      %s\n", extractorModel)
			} else {
				fmt.Printf("  Extractor:      (defaults to model)\n")
			}
			if endpoint != "" {
				fmt.Printf("  Endpoint:       %s\n", endpoint)
			} else {
				fmt.Printf("  Endpoint:       https://api.openai.com/v1/chat/completions (default)\n")
			}
			fmt.Printf("  Key:            %s***\n", maskKey(apiKey))
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "key", "", "API key (required)")
	cmd.Flags().StringVar(&model, "model", "", "Model name, e.g. gpt-4o-mini, llama3.2 (optional)")
	cmd.Flags().StringVar(&extractorModel, "extractor-model", "", "Model name for memory extraction (optional)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "API endpoint URL (default: OpenAI)")
	return cmd
}

func providerShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current provider configuration for the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			globalDB, profileDB, activeProfile, err := openBothDBs(ctx)
			if err != nil {
				return err
			}
			defer func() {
				_ = globalDB.Close()
			}()
			defer func() {
				_ = profileDB.Close()
			}()

			repo := store.NewProviderConfigRepo(profileDB)
			cfg, err := repo.GetActive(ctx, activeProfile.ID)
			if err != nil {
				fmt.Printf("No provider configured for profile %q.\n", activeProfile.Name)
				fmt.Println("Run: noto provider set --key <key>")
				return nil
			}

			passphrase, _ := security.MachinePassphrase()
			decrypted, decErr := security.Decrypt(cfg.CredentialRef, passphrase)
			keyDisplay := "(decryption failed)"
			if decErr == nil {
				keyDisplay = maskKey(decrypted) + "***"
			}

			endpoint := cfg.Endpoint
			if endpoint == "" {
				endpoint = "https://api.openai.com/v1/chat/completions (default)"
			}

			fmt.Printf("Provider configuration for profile %q:\n", activeProfile.Name)
			fmt.Printf("  Type:      %s\n", cfg.ProviderType)
			fmt.Printf("  Model:     %s\n", cfg.EffectiveModel())
			fmt.Printf("  Extractor: %s\n", cfg.EffectiveExtractorModel())
			fmt.Printf("  Endpoint:  %s\n", endpoint)
			fmt.Printf("  Key:       %s\n", keyDisplay)
			return nil
		},
	}
}

func providerClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove the provider configuration for the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			globalDB, profileDB, activeProfile, err := openBothDBs(ctx)
			if err != nil {
				return err
			}
			defer func() {
				_ = globalDB.Close()
			}()
			defer func() {
				_ = profileDB.Close()
			}()

			if err := deactivateProviderConfigs(ctx, profileDB, activeProfile.ID); err != nil {
				return err
			}
			fmt.Printf("Provider configuration cleared for profile %q.\n", activeProfile.Name)
			return nil
		},
	}
}

func providerExtractorModelCmd() *cobra.Command {
	var model string
	return &cobra.Command{
		Use:   "extractor-model",
		Short: "Set the extractor model for the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			model = args[0]
			ctx := context.Background()
			globalDB, profileDB, activeProfile, err := openBothDBs(ctx)
			if err != nil {
				return err
			}
			defer func() {
				_ = globalDB.Close()
			}()
			defer func() {
				_ = profileDB.Close()
			}()

			repo := store.NewProviderConfigRepo(profileDB)
			if err := repo.SetExtractorModel(ctx, activeProfile.ID, model); err != nil {
				return err
			}
			fmt.Printf("Extractor model set for profile %q: %s\n", activeProfile.Name, model)
			return nil
		},
	}
}

// ---- helpers ----------------------------------------------------------------

// openBothDBs opens the global DB, resolves the active profile, then opens
// the profile DB. Returns all three for use in command handlers.
func openBothDBs(ctx context.Context) (*store.DB, *store.DB, *store.Profile, error) {
	globalDB, err := openGlobalDB()
	if err != nil {
		return nil, nil, nil, err
	}

	activeProfile, err := resolveActiveProfile(ctx, globalDB)
	if err != nil {
		_ = globalDB.Close()
		return nil, nil, nil, err
	}

	profileDB, err := openProfileDB(activeProfile.Slug)
	if err != nil {
		_ = globalDB.Close()
		return nil, nil, nil, fmt.Errorf("provider: open profile db: %w", err)
	}

	return globalDB, profileDB, activeProfile, nil
}

func resolveActiveProfile(ctx context.Context, db *store.DB) (*store.Profile, error) {
	svc := profileServiceAdapter{repo: store.NewProfileRepo(db)}
	p, err := svc.GetActive(ctx)
	if err != nil {
		return nil, errors.New("no active profile — run: noto profile select <name>")
	}
	return p, nil
}

func deactivateProviderConfigs(ctx context.Context, db *store.DB, profileID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE provider_config SET is_active = 0 WHERE profile_id = ?`, profileID)
	return err
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:4]
}

type profileServiceAdapter struct {
	repo *store.ProfileRepo
}

func (a profileServiceAdapter) GetActive(ctx context.Context) (*store.Profile, error) {
	return a.repo.GetDefault(ctx)
}
