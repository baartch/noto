package commands

// RegisterModelCommand registers the /model command into r.
// The handler is a no-op because the TUI intercepts "/model" before the
// dispatcher executes it, opening the interactive model picker directly.
func RegisterModelCommand(r *Registry) error {
	return r.Register(&Command{
		Path:        "model",
		Usage:       "model",
		Description: "Switch the active AI model (opens interactive picker)",
		Scope:       ScopeProfile,
		Handler: func(_ *ExecContext, _ []string) error {
			// Intercepted by the TUI before this handler is ever called.
			return nil
		},
	})
}
