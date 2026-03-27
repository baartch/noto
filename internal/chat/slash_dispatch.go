package chat

import (
	"fmt"
	"strings"

	"noto/internal/commands"
	"noto/internal/parser"
	"noto/internal/suggest"
)

// DispatchResult describes the outcome of processing a chat input line.
type DispatchResult struct {
	// IsSlash indicates the input was a slash command attempt.
	IsSlash bool

	// Executed is true if a command was successfully found and run.
	Executed bool

	// Output is any text output produced by the command (may be empty).
	Output string

	// Suggestions are returned when the input is partial slash input.
	Suggestions []suggest.Suggestion

	// Err is set if command execution failed or the slash command was unknown.
	Err error
}

// Dispatcher routes chat input lines to either the command registry or the chat pipeline.
type Dispatcher struct {
	registry *commands.Registry
	engine   *suggest.Engine
}

// NewDispatcher creates a Dispatcher.
func NewDispatcher(registry *commands.Registry) *Dispatcher {
	return &Dispatcher{
		registry: registry,
		engine:   suggest.New(registry),
	}
}

// Dispatch processes a raw input line. Returns a DispatchResult indicating what happened.
func (d *Dispatcher) Dispatch(input string, ctx *commands.ExecContext) DispatchResult {
	if !parser.IsSlashInput(input) {
		return DispatchResult{IsSlash: false}
	}

	parsed := parser.Parse(input)

	// Partial input: show suggestions.
	if parsed.Partial || parsed.CommandPath == "" {
		prefix := parser.PrefixFromInput(input)
		sug := d.engine.Suggest(prefix)
		return DispatchResult{
			IsSlash:     true,
			Suggestions: sug,
		}
	}

	// Look up command.
	cmd, found := d.registry.Lookup(parsed.CommandPath)
	if !found {
		// Unknown command — return error with suggestions.
		prefix := parser.PrefixFromInput(input)
		sug := d.engine.Suggest(prefix)
		return DispatchResult{
			IsSlash:     true,
			Suggestions: sug,
			Err:         fmt.Errorf("unknown command %q. Did you mean: %s", "/"+parsed.CommandPath, formatSuggestions(sug)),
		}
	}

	// Execute with confirmation if required.
	if cmd.RequiresConfirmation && ctx.Confirm == nil {
		return DispatchResult{
			IsSlash: true,
			Err:     fmt.Errorf("command %q requires confirmation but no confirm function is available", "/"+parsed.CommandPath),
		}
	}

	var out strings.Builder
	execCtx := &commands.ExecContext{
		ProfileID:   ctx.ProfileID,
		ProfileSlug: ctx.ProfileSlug,
		Output:      &out,
		Confirm:     ctx.Confirm,
	}

	if err := cmd.Handler(execCtx, parsed.Args); err != nil {
		return DispatchResult{
			IsSlash: true,
			Err:     err,
		}
	}

	return DispatchResult{
		IsSlash:  true,
		Executed: true,
		Output:   out.String(),
	}
}

func formatSuggestions(sug []suggest.Suggestion) string {
	if len(sug) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(sug))
	for _, s := range sug {
		parts = append(parts, "/"+s.CommandPath)
	}
	return strings.Join(parts, ", ")
}
