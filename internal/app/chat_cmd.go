package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

func chatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session with the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Starting chat session... (TUI not yet wired in this build)")
			return nil
		},
	}
}
