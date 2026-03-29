package app

import (
	"fmt"
	"io"
	"os"

	"noto/internal/commands"
	"noto/internal/config"
	"noto/internal/observe"
	"noto/internal/store"
)

// Container holds all wired dependencies available to the running application.
type Container struct {
	DB       *store.DB
	Registry *commands.Registry
	Logger   observe.Logger
	Metrics  observe.MetricsEmitter
}

// Bootstrap initializes the application for the given profile slug.
// It creates required directories, opens the SQLite database, applies migrations,
// registers all canonical commands, and wires the logger and metrics emitter.
func Bootstrap(slug string, logOutput io.Writer) (*Container, error) {
	// Ensure all profile directories exist.
	if err := config.EnsureAppDirs(slug); err != nil {
		return nil, fmt.Errorf("app: ensure dirs: %w", err)
	}

	// Open the profile-scoped SQLite database.
	dbPath, err := config.ProfileDBPath(slug)
	if err != nil {
		return nil, fmt.Errorf("app: db path: %w", err)
	}
	db, err := store.OpenProfile(dbPath)
	if err != nil {
		return nil, fmt.Errorf("app: open db: %w", err)
	}

	// Wire logging.
	if logOutput == nil {
		logOutput = io.Discard
	}
	logger := observe.NewJSONLogger(logOutput)

	// Build the shared command registry.
	registry := commands.NewRegistry()

	return &Container{
		DB:       db,
		Registry: registry,
		Logger:   logger,
		Metrics:  observe.NoopMetrics{},
	}, nil
}

// Run is the top-level entry point called from main. It performs minimal bootstrapping
// for the global/setup phase (no profile selected yet) then hands off to the TUI or CLI.
func Run() {
	// Ensure the base app directory exists.
	appDir, err := config.AppDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "noto: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "noto: create app dir: %v\n", err)
		os.Exit(1)
	}

	// Hand off to the CLI root command.
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
