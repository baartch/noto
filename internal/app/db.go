package app

import (
	"noto/internal/config"
	"noto/internal/store"
)

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
