package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// AppDirName is the top-level directory under the user's home.
	AppDirName = ".noto"

	// ProfilesDirName is the subdirectory containing per-profile directories.
	ProfilesDirName = "profiles"

	// MemoryDBName is the SQLite database filename for each profile.
	MemoryDBName = "memory.db"

	// VectorIndexName is the single-file vector index filename for each profile.
	VectorIndexName = "memory.vec"

	// BackupsDirName is the subdirectory within each profile for backup snapshots.
	BackupsDirName = "backups"

	// PromptsDirName is the subdirectory within each profile for prompt files.
	PromptsDirName = "prompts"

	// SystemPromptName is the default system prompt filename.
	SystemPromptName = "system.md"

	// CacheDirName is the subdirectory within each profile for cached context.
	CacheDirName = "cache"

	// ActiveProfileFile is the file that records the currently active profile slug.
	ActiveProfileFile = "active_profile"
)

// AppDir returns the absolute path to the top-level Noto application directory (~/.noto).
func AppDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: could not determine home directory: %w", err)
	}
	return filepath.Join(home, AppDirName), nil
}

// ProfilesDir returns the absolute path to the profiles root directory.
func ProfilesDir() (string, error) {
	app, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(app, ProfilesDirName), nil
}

// ProfileDir returns the absolute path to the directory for the given profile slug.
func ProfileDir(slug string) (string, error) {
	profiles, err := ProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(profiles, slug), nil
}

// ProfileDBPath returns the absolute path to the SQLite memory database for a profile.
func ProfileDBPath(slug string) (string, error) {
	dir, err := ProfileDir(slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, MemoryDBName), nil
}

// ProfileVectorPath returns the absolute path to the vector index file for a profile.
func ProfileVectorPath(slug string) (string, error) {
	dir, err := ProfileDir(slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, VectorIndexName), nil
}

// ProfileBackupsDir returns the absolute path to the backups directory for a profile.
func ProfileBackupsDir(slug string) (string, error) {
	dir, err := ProfileDir(slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, BackupsDirName), nil
}

// ProfilePromptsDir returns the absolute path to the prompts directory for a profile.
func ProfilePromptsDir(slug string) (string, error) {
	dir, err := ProfileDir(slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, PromptsDirName), nil
}

// ProfileSystemPromptPath returns the absolute path to the system prompt file for a profile.
func ProfileSystemPromptPath(slug string) (string, error) {
	dir, err := ProfilePromptsDir(slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, SystemPromptName), nil
}

// ActiveProfilePath returns the absolute path to the active-profile marker file.
func ActiveProfilePath() (string, error) {
	app, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(app, ActiveProfileFile), nil
}

// EnsureAppDirs creates all required application directories for the given profile slug.
func EnsureAppDirs(slug string) error {
	dirs := []func(string) (string, error){
		ProfileDir,
		ProfileBackupsDir,
		ProfilePromptsDir,
	}
	for _, fn := range dirs {
		path, err := fn(slug)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("config: could not create directory %s: %w", path, err)
		}
	}
	return nil
}
