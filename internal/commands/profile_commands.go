package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"noto/internal/store"
)

// ProfileService is the subset of profile.Service used by command handlers.
type ProfileService interface {
	Create(ctx context.Context, name string) (*store.Profile, error)
	List(ctx context.Context) ([]*store.Profile, error)
	Select(ctx context.Context, name string) (*store.Profile, error)
	Rename(ctx context.Context, oldName, newName string) (*store.Profile, error)
	Delete(ctx context.Context, name string, confirm func(string) bool) error
	GetActive(ctx context.Context) (*store.Profile, error)
}

// RegisterProfileCommands registers all profile-related commands using svc for
// DB operations. Pass a real *profile.Service from the wiring layer.
func RegisterProfileCommands(r *Registry, svc ProfileService) error {
	cmds := []*Command{
		{
			Path:        "profile create",
			Usage:       "profile create <name>",
			Description: "Create a new profile with the given name",
			Scope:       ScopeGlobal,
			Handler:     profileCreateHandler(svc),
		},
		{
			Path:        "profile list",
			Usage:       "profile list",
			Description: "List all profiles",
			Scope:       ScopeGlobal,
			Handler:     profileListHandler(svc),
		},
		{
			Path:        "profile select",
			Usage:       "profile select <name>",
			Description: "Select a profile as the active profile",
			Scope:       ScopeGlobal,
			Handler:     profileSelectHandler(svc),
		},
		{
			Path:        "profile rename",
			Usage:       "profile rename <old> <new>",
			Description: "Rename a profile",
			Scope:       ScopeGlobal,
			Handler:     profileRenameHandler(svc),
		},
		{
			Path:                 "profile delete",
			Usage:                "profile delete <name>",
			Description:          "Permanently delete a profile and all its data",
			RequiresConfirmation: true,
			Scope:                ScopeGlobal,
			Handler:              profileDeleteHandler(svc),
		},
	}
	for _, cmd := range cmds {
		if err := r.Register(cmd); err != nil {
			return err
		}
	}
	return nil
}

func profileCreateHandler(svc ProfileService) HandlerFunc {
	return func(ctx *ExecContext, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: profile create <name>")
		}
		name := strings.Join(args, " ")
		p, err := svc.Create(context.Background(), name)
		if err != nil {
			return err
		}
		fmt.Fprintf(ctx.Output, "Created profile %q (slug: %s)\n", p.Name, p.Slug)
		return nil
	}
}

func profileListHandler(svc ProfileService) HandlerFunc {
	return func(ctx *ExecContext, _ []string) error {
		profiles, err := svc.List(context.Background())
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(ctx.Output, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSLUG\tACTIVE")
		for _, p := range profiles {
			active := ""
			if p.IsDefault {
				active = "●"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Slug, active)
		}
		return w.Flush()
	}
}

func profileSelectHandler(svc ProfileService) HandlerFunc {
	return func(ctx *ExecContext, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: profile select <name>")
		}
		p, err := svc.Select(context.Background(), args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(ctx.Output, "Active profile: %q\n", p.Name)
		if ctx.OnProfileChanged != nil {
			ctx.OnProfileChanged(p.Name)
		}
		return nil
	}
}

func profileRenameHandler(svc ProfileService) HandlerFunc {
	return func(ctx *ExecContext, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("usage: profile rename <old> <new>")
		}
		p, err := svc.Rename(context.Background(), args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(ctx.Output, "Renamed to %q\n", p.Name)
		// If we renamed the currently active profile, update the header.
		if ctx.OnProfileChanged != nil {
			active, aerr := svc.GetActive(context.Background())
			if aerr == nil {
				ctx.OnProfileChanged(active.Name)
			}
		}
		return nil
	}
}

func profileDeleteHandler(svc ProfileService) HandlerFunc {
	return func(ctx *ExecContext, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: profile delete <name>")
		}
		name := args[0]
		err := svc.Delete(context.Background(), name, func(prompt string) bool {
			if ctx.Confirm != nil {
				return ctx.Confirm(prompt)
			}
			return false
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(ctx.Output, "Deleted profile %q\n", name)
		// After deletion, report whichever profile is now active.
		if ctx.OnProfileChanged != nil {
			active, aerr := svc.GetActive(context.Background())
			if aerr == nil {
				ctx.OnProfileChanged(active.Name)
			}
		}
		return nil
	}
}

// HandlerFunc is the function signature for command handlers.
type HandlerFunc = func(ctx *ExecContext, args []string) error
