package commands

import (
	"fmt"
)

// RegisterPromptCommands registers all prompt-related commands into r.
func RegisterPromptCommands(r *Registry) error {
	cmds := []*Command{
		{
			Path:        "prompt show",
			Usage:       "prompt show",
			Description: "Display the current system prompt for the active profile",
			Scope:       ScopeProfile,
			Handler:     promptShowHandler,
		},
		{
			Path:        "prompt edit",
			Usage:       "prompt edit",
			Description: "Open an editor to modify the system prompt for the active profile",
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
	fmt.Fprintln(ctx.Output, "[system prompt content would appear here]")
	return nil
}

func promptEditHandler(ctx *ExecContext, _ []string) error {
	fmt.Fprintln(ctx.Output, "Opening editor for system prompt...")
	return nil
}
