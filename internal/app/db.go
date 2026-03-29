package app

import (
	"fmt"
	"os"

	"noto/internal/config"
	"noto/internal/store"
)

// openGlobalDB opens (or creates) the global registry database at ~/.noto/global.db.
// The global DB holds only shared registry data (no profile metadata).
func openGlobalDB() (*store.DB, error) {
	appDir, err := config.AppDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("app: create app dir: %w", err)
	}
	return store.OpenGlobal(appDir + "/global.db")
}

// openProfileDB opens (or creates) the per-profile database at
// ~/.noto/profiles/<slug>/memory.db.
func openProfileDB(slug string) (*store.DB, error) {
	if err := config.EnsureAppDirs(slug); err != nil {
		return nil, err
	}
	dbPath, err := config.ProfileDBPath(slug)
	if err != nil {
		return nil, err
	}
	return store.OpenProfile(dbPath)
}
