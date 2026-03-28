package app

import (
	"context"
	"fmt"
	"io"

	"noto/internal/profile"
	"noto/internal/store"
)

// StartupResult describes the outcome of startup profile resolution.
type StartupResult struct {
	// Profile is the resolved active profile.
	Profile *store.Profile

	// Action describes what happened: "selected_default", "selected_only", "prompt_create".
	Action string
}

// StartupFlow resolves which profile to use at application startup.
type StartupFlow struct {
	profileSvc *profile.Service
}

// NewStartupFlow creates a StartupFlow.
func NewStartupFlow(svc *profile.Service) *StartupFlow {
	return &StartupFlow{profileSvc: svc}
}

// Resolve runs the startup profile resolution logic:
//  - Zero profiles: prompts to create the first profile.
//  - One profile: selects it automatically.
//  - Multiple profiles: uses the default profile if set; otherwise selects the last-used profile.
func (f *StartupFlow) Resolve(
	ctx context.Context,
	w io.Writer,
	promptCreateName func() (string, error),
	promptSelectName func([]*store.Profile) (string, error),
) (*StartupResult, error) {
	_ = promptSelectName
	profiles, err := f.profileSvc.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("startup: list profiles: %w", err)
	}

	switch len(profiles) {
	case 0:
		// No profiles — must create one.
		fmt.Fprintln(w, "Welcome to Noto! Let's create your first profile.")
		name, err := promptCreateName()
		if err != nil {
			return nil, err
		}
		p, err := f.profileSvc.Create(ctx, name)
		if err != nil {
			return nil, err
		}
		if _, err := f.profileSvc.Select(ctx, p.Name); err != nil {
			return nil, err
		}
		p.IsDefault = true
		return &StartupResult{Profile: p, Action: "prompt_create"}, nil

	case 1:
		// Single profile — select it automatically.
		p := profiles[0]
		if !p.IsDefault {
			if _, err := f.profileSvc.Select(ctx, p.Name); err != nil {
				return nil, err
			}
			p.IsDefault = true
		}
		return &StartupResult{Profile: p, Action: "selected_only"}, nil

	default:
		// Multiple profiles — use default if available.
		for _, p := range profiles {
			if p.IsDefault {
				return &StartupResult{Profile: p, Action: "selected_default"}, nil
			}
		}
		// No default set — select the last-used profile.
		p, err := f.profileSvc.LastUsed(ctx)
		if err != nil {
			return nil, err
		}
		selected, err := f.profileSvc.Select(ctx, p.Name)
		if err != nil {
			return nil, err
		}
		return &StartupResult{Profile: selected, Action: "selected_last_used"}, nil
	}
}
