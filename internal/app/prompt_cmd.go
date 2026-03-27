package app

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"noto/internal/profile"
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
			// TODO: resolve active profile slug from DB; using "default" as placeholder.
			slug := resolveActiveSlug()
			ps := profile.NewPromptStore(slug)
			content, err := ps.GetSystemPrompt()
			if err != nil {
				return err
			}
			fmt.Println(content)
			return nil
		},
	}
}

func promptEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the system prompt in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := resolveActiveSlug()
			ps := profile.NewPromptStore(slug)

			// Ensure file exists.
			if _, err := ps.GetSystemPrompt(); err != nil {
				return err
			}

			home, _ := os.UserHomeDir()
			path := home + "/.noto/profiles/" + slug + "/prompts/system.md"

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}

// resolveActiveSlug returns the active profile slug.
// TODO: read from ~/.noto/active_profile or DB.
func resolveActiveSlug() string {
	data, err := os.ReadFile(os.Getenv("HOME") + "/.noto/active_profile")
	if err != nil || len(data) == 0 {
		return "default"
	}
	slug := string(data)
	for len(slug) > 0 && (slug[len(slug)-1] == '\n' || slug[len(slug)-1] == '\r') {
		slug = slug[:len(slug)-1]
	}
	return slug
}
