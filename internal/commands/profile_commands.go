package commands

import (
	"fmt"
	"strings"
)

// RegisterProfileCommands registers all profile-related commands into r.
func RegisterProfileCommands(r *Registry) error {
	cmds := []*Command{
		{
			Path:        "profile create",
			Usage:       "profile create <name>",
			Description: "Create a new profile with the given name",
			Scope:       ScopeGlobal,
			Handler:     profileCreateHandler,
		},
		{
			Path:        "profile list",
			Usage:       "profile list",
			Description: "List all profiles",
			Scope:       ScopeGlobal,
			Handler:     profileListHandler,
		},
		{
			Path:        "profile select",
			Usage:       "profile select <name>",
			Description: "Select a profile as the active profile",
			Scope:       ScopeGlobal,
			Handler:     profileSelectHandler,
		},
		{
			Path:        "profile rename",
			Usage:       "profile rename <old> <new>",
			Description: "Rename a profile",
			Scope:       ScopeGlobal,
			Handler:     profileRenameHandler,
		},
		{
			Path:                 "profile delete",
			Usage:                "profile delete <name>",
			Description:          "Permanently delete a profile and all its data",
			RequiresConfirmation: true,
			Scope:                ScopeGlobal,
			Handler:              profileDeleteHandler,
		},
	}
	for _, cmd := range cmds {
		if err := r.Register(cmd); err != nil {
			return err
		}
	}
	return nil
}

func profileCreateHandler(ctx *ExecContext, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: profile create <name>")
	}
	name := strings.Join(args, " ")
	fmt.Fprintf(ctx.Output, "Created profile: %s\n", name)
	return nil
}

func profileListHandler(ctx *ExecContext, _ []string) error {
	fmt.Fprintln(ctx.Output, "Listing profiles...")
	return nil
}

func profileSelectHandler(ctx *ExecContext, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: profile select <name>")
	}
	fmt.Fprintf(ctx.Output, "Selected profile: %s\n", args[0])
	return nil
}

func profileRenameHandler(ctx *ExecContext, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: profile rename <old> <new>")
	}
	fmt.Fprintf(ctx.Output, "Renamed profile %q to %q\n", args[0], args[1])
	return nil
}

func profileDeleteHandler(ctx *ExecContext, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: profile delete <name>")
	}
	name := args[0]
	if ctx.Confirm != nil {
		msg := fmt.Sprintf("Permanently delete profile %q and all its data?", name)
		if !ctx.Confirm(msg) {
			return fmt.Errorf("deletion cancelled")
		}
	}
	fmt.Fprintf(ctx.Output, "Deleted profile: %s\n", name)
	return nil
}
