package commands

import (
	"context"
	"errors"
	"fmt"

	"noto/internal/config"
	"noto/internal/store"
)

// ErrOpenExtractorModelPicker is a sentinel error indicating the TUI should open the extractor model picker.
type ErrOpenExtractorModelPicker struct{}

func (e *ErrOpenExtractorModelPicker) Error() string { return "open-extractor-model-picker" }

// AsErrOpenExtractorModelPicker checks whether err is an *ErrOpenExtractorModelPicker.
func AsErrOpenExtractorModelPicker(err error) bool {
	_, ok := err.(*ErrOpenExtractorModelPicker)
	return ok
}

// RegisterModelExtractorCommand registers /model extractor into r.
func RegisterModelExtractorCommand(r *Registry) error {
	return r.Register(&Command{
		Path:        "model extractor",
		Usage:       "model extractor <model>",
		Description: "Set the extractor model for the active profile",
		Scope:       ScopeProfile,
		Handler:     modelExtractorHandler,
	})
}

func modelExtractorHandler(ctx *ExecContext, args []string) error {
	if ctx.ProfileSlug == "" || ctx.ProfileID == "" {
		return errors.New("no active profile")
	}
	if len(args) == 0 {
		return &ErrOpenExtractorModelPicker{}
	}
	model := args[0]

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

	repo := store.NewProviderConfigRepo(db)
	if err := repo.SetExtractorModel(context.Background(), ctx.ProfileID, model); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ctx.Output, "Extractor model set to: %s\n", model); err != nil {
		return err
	}
	return nil
}
