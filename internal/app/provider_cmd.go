package app

import (
	"context"
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
	return cmd
}

func providerSetCmd() *cobra.Command {
	var (
		apiKey   string
		model    string
		endpoint string
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the provider configuration for the active profile",
		Example: `  noto provider set --key sk-... --model gpt-4o-mini
  noto provider set --endpoint http://localhost:11434/v1/chat/completions --model llama3.2 --key ollama`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if apiKey == "" {
				return fmt.Errorf("--key is required")
			}
			if model == "" {
				return fmt.Errorf("--model is required")
			}

			ctx := context.Background()
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer db.Close()

			// Resolve active profile.
			activeProfile, err := resolveActiveProfile(ctx, db)
			if err != nil {
				return err
			}

			// Encrypt the API key.
			passphrase, err := security.MachinePassphrase()
			if err != nil {
				return fmt.Errorf("provider: get passphrase: %w", err)
			}
			encrypted, err := security.Encrypt(apiKey, passphrase)
			if err != nil {
				return fmt.Errorf("provider: encrypt key: %w", err)
			}

			// Deactivate any existing configs for this profile first.
			if err := deactivateProviderConfigs(ctx, db, activeProfile.ID); err != nil {
				return err
			}

			cfg := &store.ProviderConfig{
				ID:            fmt.Sprintf("pc-%x", time.Now().UnixNano()),
				ProfileID:     activeProfile.ID,
				ProviderType:  "openai_compatible",
				Endpoint:      endpoint,
				Model:         model,
				CredentialRef: encrypted,
				IsActive:      true,
			}

			repo := store.NewProviderConfigRepo(db)
			if err := repo.Upsert(ctx, cfg); err != nil {
				return err
			}

			fmt.Printf("Provider configured for profile %q\n", activeProfile.Name)
			fmt.Printf("  Model:    %s\n", model)
			if endpoint != "" {
				fmt.Printf("  Endpoint: %s\n", endpoint)
			} else {
				fmt.Printf("  Endpoint: https://api.openai.com/v1/chat/completions (default)\n")
			}
			fmt.Printf("  Key:      %s***\n", maskKey(apiKey))
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "key", "", "API key (required)")
	cmd.Flags().StringVar(&model, "model", "", "Model name, e.g. gpt-4o-mini, llama3.2 (required)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "API endpoint URL (default: OpenAI)")
	return cmd
}

func providerShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current provider configuration for the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer db.Close()

			activeProfile, err := resolveActiveProfile(ctx, db)
			if err != nil {
				return err
			}

			repo := store.NewProviderConfigRepo(db)
			cfg, err := repo.GetActive(ctx, activeProfile.ID)
			if err != nil {
				fmt.Printf("No provider configured for profile %q.\n", activeProfile.Name)
				fmt.Println("Run: noto provider set --key <key> --model <model>")
				return nil
			}

			// Decrypt to show masked key.
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
			fmt.Printf("  Type:     %s\n", cfg.ProviderType)
			fmt.Printf("  Model:    %s\n", cfg.Model)
			fmt.Printf("  Endpoint: %s\n", endpoint)
			fmt.Printf("  Key:      %s\n", keyDisplay)
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
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer db.Close()

			activeProfile, err := resolveActiveProfile(ctx, db)
			if err != nil {
				return err
			}

			if err := deactivateProviderConfigs(ctx, db, activeProfile.ID); err != nil {
				return err
			}
			fmt.Printf("Provider configuration cleared for profile %q.\n", activeProfile.Name)
			return nil
		},
	}
}

// ---- helpers ----------------------------------------------------------------

func resolveActiveProfile(ctx context.Context, db *store.DB) (*store.Profile, error) {
	svc := newProfileSvcFromDB(db)
	p, err := svc.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active profile — run: noto profile select <name>")
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

// newProfileSvcFromDB creates a profile.Service from an open DB.
// Avoids importing the profile package from provider_cmd by reusing the helper
// already available in profile_cmd.go via the shared openGlobalDB pattern.
func newProfileSvcFromDB(db *store.DB) interface {
	GetActive(ctx context.Context) (*store.Profile, error)
} {
	return profileServiceAdapter{repo: store.NewProfileRepo(db)}
}

type profileServiceAdapter struct {
	repo *store.ProfileRepo
}

func (a profileServiceAdapter) GetActive(ctx context.Context) (*store.Profile, error) {
	p, err := a.repo.GetDefault(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}
