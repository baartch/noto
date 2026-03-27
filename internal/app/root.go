package app

import (
	"github.com/spf13/cobra"
)

// RootCmd returns the top-level Cobra command for Noto.
func RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "noto",
		Short: "Noto — a local-first terminal chatbot with profile-scoped memory",
		Long: `Noto is a local-first Go terminal chatbot that provides persistent,
profile-isolated memory continuity backed by SQLite.`,
		SilenceUsage: true,
	}

	root.AddCommand(profileCmd())
	root.AddCommand(chatCmd())
	root.AddCommand(promptCmd())

	return root
}
