//go:build !production

package store

// OpenForTesting opens a single SQLite database and applies the profile-scoped
// migrations. This is only for use in tests where the global/profile DB split
// is not needed.
func OpenForTesting(path string) (*DB, error) {
	return open(path, profileMigrationsFS, "migrations/profile")
}
