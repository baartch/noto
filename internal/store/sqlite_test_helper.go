//go:build !production

package store

// OpenForTesting opens a single SQLite database and applies both the global
// (profiles) and profile-scoped migrations. This is only for use in tests
// where the global/profile DB split is not needed.
func OpenForTesting(path string) (*DB, error) {
	db, err := open(path, globalMigrationsFS, "migrations/global")
	if err != nil {
		return nil, err
	}
	// Also apply profile migrations to the same DB.
	if err := db.migrate(profileMigrationsFS, "migrations/profile"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
