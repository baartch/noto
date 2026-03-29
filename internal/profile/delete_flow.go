package profile

import (
	"context"
	"fmt"
	"io"

	"noto/internal/store"
)

// DeleteFlow orchestrates the full profile deletion UX: confirmation, reassignment, and removal.
type DeleteFlow struct {
	service *Service
}

// NewDeleteFlow creates a new DeleteFlow.
func NewDeleteFlow(service *Service) *DeleteFlow {
	return &DeleteFlow{service: service}
}

// Run executes the deletion flow, writing prompts to w and reading input from r.
// Returns an error if deletion is cancelled or fails.
func (f *DeleteFlow) Run(ctx context.Context, profileName string, w io.Writer, r io.Reader) error {
	confirm := func(_ string) bool {
		return ConfirmDeletion(w, r, profileName)
	}
	if err := f.service.Delete(ctx, profileName, confirm); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Profile %q deleted.\n", profileName); err != nil {
		return err
	}
	return nil
}

// RunNonInteractive deletes a profile without interactive confirmation.
// This is intended for automated test scenarios only — never call in production UX paths.
func (f *DeleteFlow) RunNonInteractive(ctx context.Context, profileName string) error {
	return f.service.Delete(ctx, profileName, func(_ string) bool { return true })
}

// SelectFallback chooses an appropriate active profile after deletion.
// It returns the first available profile from the list (excluding the deleted one).
func SelectFallback(profiles []*store.Profile, deletedID string) *store.Profile {
	for _, p := range profiles {
		if p.ID != deletedID {
			return p
		}
	}
	return nil
}
