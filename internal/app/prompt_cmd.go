package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"noto/internal/commands"
	"noto/internal/profile"
	"noto/internal/store"
)

func promptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Manage the system prompt for the active profile",
	}
	cmd.AddCommand(promptShowCmd())
	cmd.AddCommand(promptEditCmd())
	return cmd
}

func promptShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current system prompt",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			return runPromptCommand(ctx, "prompt show", nil)
		},
	}
}

func promptEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the system prompt in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			return runPromptCommand(ctx, "prompt edit", nil)
		},
	}
}

func runPromptCommand(ctx context.Context, commandPath string, args []string) error {
	profSvc := profile.NewService(nil)
	activeProfile, err := profSvc.GetActive(ctx)
	if err != nil {
		return err
	}

	profileDB, err := openProfileDB(activeProfile.Slug)
	if err != nil {
		return err
	}
	defer func() {
		if err := profileDB.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "prompt: close profile db: %v\n", err)
		}
	}()

	registry := commands.NewRegistry()
	if err := commands.RegisterPromptCommands(registry); err != nil {
		return err
	}

	cmd, found := registry.Lookup(commandPath)
	if !found {
		return fmt.Errorf("unknown command: %s", commandPath)
	}

	execCtx := &commands.ExecContext{
		ProfileID:        activeProfile.ID,
		ProfileSlug:      activeProfile.Slug,
		Output:           os.Stdout,
		SuspendForEditor: func(fn func() error) error { return fn() },
		OnPromptChanged: func(slug string) error {
			cacheRepo := store.NewContextCacheRepo(profileDB)
			_ = cacheRepo.InvalidateAll(ctx, activeProfile.ID)
			_ = store.NewVectorManifestRepo(profileDB).SetManifestStatusStr(ctx, activeProfile.ID, "stale")
			return nil
		},
	}

	return cmd.Handler(execCtx, args)
}
