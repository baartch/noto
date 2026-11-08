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

	if ctx.ListModels == nil {
		return errors.New("model list unavailable — cannot verify structured outputs support")
	}
	models, err := ctx.ListModels(context.Background())
	if err != nil {
		return fmt.Errorf("could not verify model support: %w", err)
	}
	found := false
	for _, mi := range models {
		if mi.ID == model {
			found = true
			if !mi.SupportsStructuredOutputs {
				return fmt.Errorf("model %s does not support structured outputs — extraction requires it", model)
			}
			break
		}
	}
	if !found {
		return fmt.Errorf("model %s not found in provider model list", model)
	}

	path, err := config.ProfileDBPath(ctx.ProfileSlug)
	if err != nil {
		return err
	}
	db, err := store.OpenProfileLogged(path)
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
