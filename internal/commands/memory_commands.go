package commands

import (
	"context"
	"fmt"
	"strings"

	"noto/internal/config"
	"noto/internal/store"
)

// RegisterMemoryCommands registers memory-related commands into r.
func RegisterMemoryCommands(r *Registry) error {
	return r.Register(&Command{
		Path:        "memory edit",
		Usage:       "memory edit <note-id> <content>",
		Description: "Edit a memory note",
		Scope:       ScopeProfile,
		Handler:     memoryEditHandler,
	})
}

func memoryEditHandler(ctx *ExecContext, args []string) error {
	if ctx.ProfileSlug == "" || ctx.ProfileID == "" {
		return fmt.Errorf("no active profile")
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: memory edit <note-id> <content>")
	}

	noteID := args[0]
	content := strings.Join(args[1:], " ")
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("content must not be empty")
	}

	path, err := config.ProfileDBPath(ctx.ProfileSlug)
	if err != nil {
		return err
	}
	db, err := store.OpenProfile(path)
	if err != nil {
		return err
	}
	defer db.Close()

	repo := store.NewMemoryNoteRepo(db)
	note, err := repo.GetByID(context.Background(), noteID)
	if err != nil {
		return err
	}
	if note.ProfileID != ctx.ProfileID {
		return fmt.Errorf("note does not belong to active profile")
	}

	note.Content = content
	if err := repo.Update(context.Background(), note); err != nil {
		return err
	}

	if ctx.OnPromptChanged != nil {
		_ = ctx.OnPromptChanged(ctx.ProfileSlug)
	}
	fmt.Fprintf(ctx.Output, "Updated note %s\n", noteID)
	return nil
}
