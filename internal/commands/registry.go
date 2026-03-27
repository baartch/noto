package commands

import (
	"fmt"
	"sort"
	"strings"
)

// Scope defines whether a command operates on a specific profile or globally.
type Scope string

const (
	ScopeProfile Scope = "profile"
	ScopeGlobal  Scope = "global"
)

// Command represents a single canonical command definition shared by the CLI and slash dispatcher.
type Command struct {
	// Path is the canonical hierarchical path, e.g. "profile list" or "prompt show".
	Path string

	// Usage is the short usage string, e.g. "profile list".
	Usage string

	// Description is the human-readable description shown in suggestions and help.
	Description string

	// Aliases are alternative paths that also resolve to this command.
	Aliases []string

	// RequiresConfirmation indicates that the command must request explicit user confirmation.
	RequiresConfirmation bool

	// Scope indicates whether the command acts within a profile or globally.
	Scope Scope

	// Handler is the function invoked when the command is executed.
	// args contains positional arguments after the command path.
	Handler func(ctx *ExecContext, args []string) error
}

// ExecContext carries the runtime context for command execution.
type ExecContext struct {
	// ProfileID is the active profile's ID at execution time (may be empty for global commands).
	ProfileID string

	// ProfileSlug is the active profile slug.
	ProfileSlug string

	// Output is the writer where command output should be sent.
	Output interface{ Write([]byte) (int, error) }

	// Confirm is a function that prompts the user for explicit confirmation.
	// It returns true if the user confirmed, false otherwise.
	Confirm func(prompt string) bool
}

// Registry holds all registered commands and provides lookup and listing operations.
type Registry struct {
	commands map[string]*Command // keyed by canonical path
	aliases  map[string]string   // alias → canonical path
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
		aliases:  make(map[string]string),
	}
}

// Register adds a command to the registry. Returns an error on path or alias collision.
func (r *Registry) Register(cmd *Command) error {
	if cmd.Path == "" {
		return fmt.Errorf("commands: command path must not be empty")
	}
	if _, exists := r.commands[cmd.Path]; exists {
		return fmt.Errorf("commands: duplicate command path %q", cmd.Path)
	}
	r.commands[cmd.Path] = cmd

	for _, alias := range cmd.Aliases {
		if existing, ok := r.aliases[alias]; ok {
			return fmt.Errorf("commands: alias %q already registered for %q", alias, existing)
		}
		r.aliases[alias] = cmd.Path
	}
	return nil
}

// Lookup resolves a command by exact canonical path or alias.
func (r *Registry) Lookup(path string) (*Command, bool) {
	if cmd, ok := r.commands[path]; ok {
		return cmd, true
	}
	if canonical, ok := r.aliases[path]; ok {
		cmd, found := r.commands[canonical]
		return cmd, found
	}
	return nil, false
}

// PrefixMatches returns all commands whose Path begins with the given prefix (case-insensitive).
// Results are sorted by path for deterministic ordering.
func (r *Registry) PrefixMatches(prefix string) []*Command {
	lower := strings.ToLower(prefix)
	var matches []*Command
	for path, cmd := range r.commands {
		if strings.HasPrefix(strings.ToLower(path), lower) {
			matches = append(matches, cmd)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Path < matches[j].Path
	})
	return matches
}

// All returns all registered commands sorted by path.
func (r *Registry) All() []*Command {
	cmds := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Path < cmds[j].Path
	})
	return cmds
}
