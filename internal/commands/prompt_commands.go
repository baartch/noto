package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"noto/internal/profile"
)

// ErrOpenEditor is returned by promptEditHandler to signal the TUI that it
// should open an editor via tea.ExecProcess. The Path field holds the file.
type ErrOpenEditor struct {
	Path string
}

func (e *ErrOpenEditor) Error() string { return "open-editor:" + e.Path }

// AsErrOpenEditor unwraps err as *ErrOpenEditor if it is one.
func AsErrOpenEditor(err error) (*ErrOpenEditor, bool) {
	var e *ErrOpenEditor
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// RegisterPromptCommands registers all prompt-related commands into r.
func RegisterPromptCommands(r *Registry) error {
	cmds := []*Command{
		{
			Path:        "prompt show",
			Usage:       "prompt show",
			Description: "Show the system prompt for the active profile",
			Scope:       ScopeProfile,
			Handler:     promptShowHandler,
		},
		{
			Path:        "prompt edit",
			Usage:       "prompt edit",
			Description: "Edit the system prompt for the active profile in $EDITOR",
			Scope:       ScopeProfile,
			Handler:     promptEditHandler,
		},
	}
	for _, cmd := range cmds {
		if err := r.Register(cmd); err != nil {
			return err
		}
	}
	return nil
}

func promptShowHandler(ctx *ExecContext, _ []string) error {
	if ctx.ProfileSlug == "" {
		return fmt.Errorf("no active profile")
	}
	ps := profile.NewPromptStore(ctx.ProfileSlug)
	content, err := ps.GetSystemPrompt()
	if err != nil {
		return fmt.Errorf("prompt show: %w", err)
	}
	if _, err := fmt.Fprintln(ctx.Output, content); err != nil {
		return err
	}
	return nil
}

func promptEditHandler(ctx *ExecContext, _ []string) error {
	if ctx.ProfileSlug == "" {
		return fmt.Errorf("no active profile")
	}

	ps := profile.NewPromptStore(ctx.ProfileSlug)

	// Ensure the file exists.
	if _, err := ps.GetSystemPrompt(); err != nil {
		return fmt.Errorf("prompt edit: %w", err)
	}

	promptPath, err := promptFilePath(ctx.ProfileSlug)
	if err != nil {
		return err
	}

	if ctx.SuspendForEditor == nil {
		// CLI / non-TUI mode — exec directly.
		if err := openInEditor(promptPath); err != nil {
			return err
		}
		if ctx.OnPromptChanged != nil {
			return ctx.OnPromptChanged(ctx.ProfileSlug)
		}
		return nil
	}

	// TUI mode — signal via sentinel error so the TUI can use tea.ExecProcess.
	return &ErrOpenEditor{Path: promptPath}
}

func openInEditor(path string) error {
	ctx := context.Background()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func promptFilePath(slug string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.noto/profiles/" + slug + "/prompts/system.md", nil
}
