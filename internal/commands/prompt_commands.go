package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"noto/internal/config"
	"noto/internal/profile"
	"noto/internal/store"
)

// ErrOpenEditor is returned by promptEditHandler to signal the TUI that it
// should open an editor via tea.ExecProcess. The Path field holds the file.
type ErrOpenEditor struct {
	Path   string
	OnSave func() error
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
	if ctx.ProfileSlug == "" || ctx.ProfileID == "" {
		return errors.New("no active profile")
	}
	path, err := config.ProfileDBPath(ctx.ProfileSlug)
	if err != nil {
		return err
	}
	db, err := store.OpenProfile(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	repo := store.NewSystemPromptRepo(db)
	ps := profile.NewPromptStore(ctx.ProfileID, repo)
	content, err := ps.GetSystemPrompt(context.Background())
	if err != nil {
		return fmt.Errorf("prompt show: %w", err)
	}
	if _, err := fmt.Fprintln(ctx.Output, content); err != nil {
		return err
	}
	return nil
}

func promptEditHandler(ctx *ExecContext, _ []string) error {
	if ctx.ProfileSlug == "" || ctx.ProfileID == "" {
		return errors.New("no active profile")
	}
	path, err := config.ProfileDBPath(ctx.ProfileSlug)
	if err != nil {
		return err
	}
	db, err := store.OpenProfile(path)
	if err != nil {
		return err
	}

	repo := store.NewSystemPromptRepo(db)
	ps := profile.NewPromptStore(ctx.ProfileID, repo)
	content, err := ps.GetSystemPrompt(context.Background())
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("prompt edit: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "noto-prompt-*.md")
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("prompt edit: %w", err)
	}
	promptPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = db.Close()
		_ = os.Remove(promptPath)
		return fmt.Errorf("prompt edit: %w", err)
	}
	_ = tmpFile.Close()

	save := func() error {
		defer func() {
			_ = os.Remove(promptPath)
			_ = db.Close()
		}()
		data, err := os.ReadFile(promptPath)
		if err != nil {
			return fmt.Errorf("prompt edit: %w", err)
		}
		if err := ps.SetSystemPrompt(context.Background(), string(data)); err != nil {
			return err
		}
		if ctx.OnPromptChanged != nil {
			return ctx.OnPromptChanged(ctx.ProfileSlug)
		}
		return nil
	}

	if ctx.SuspendForEditor == nil {
		// CLI / non-TUI mode — exec directly.
		if err := openInEditor(promptPath); err != nil {
			_ = save()
			return err
		}
		return save()
	}

	// TUI mode — signal via sentinel error so the TUI can use tea.ExecProcess.
	return &ErrOpenEditor{Path: promptPath, OnSave: save}
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
