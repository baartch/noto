package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"noto/internal/config"
)

const (
	// MaxBackups is the maximum number of backup snapshots to retain per profile.
	MaxBackups = 10

	// dbSnapshotSuffix is appended to the DB backup filename.
	dbSnapshotSuffix = ".db"

	// vecSnapshotSuffix is appended to the vector index backup filename.
	vecSnapshotSuffix = ".vec"
)

// Snapshot creates a timestamped backup of the profile's DB (and optionally vector index) files.
func Snapshot(slug string) error {
	backupsDir, err := config.ProfileBackupsDir(slug)
	if err != nil {
		return fmt.Errorf("backup: get backups dir for %q: %w", slug, err)
	}
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		return fmt.Errorf("backup: create backups dir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")

	dbSrc, err := config.ProfileDBPath(slug)
	if err != nil {
		return err
	}
	dbDst := filepath.Join(backupsDir, ts+dbSnapshotSuffix)
	if err := copyFile(dbSrc, dbDst); err != nil {
		return fmt.Errorf("backup: snapshot DB for %q: %w", slug, err)
	}

	// Vector index is optional – skip if it doesn't exist yet.
	vecSrc, err := config.ProfileVectorPath(slug)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(vecSrc); statErr == nil {
		vecDst := filepath.Join(backupsDir, ts+vecSnapshotSuffix)
		if err := copyFile(vecSrc, vecDst); err != nil {
			return fmt.Errorf("backup: snapshot vector for %q: %w", slug, err)
		}
	}

	return pruneBackups(backupsDir)
}

// Restore replaces the profile's DB (and vector index if available) with the most recent backup.
func Restore(slug string) error {
	backupsDir, err := config.ProfileBackupsDir(slug)
	if err != nil {
		return fmt.Errorf("backup: get backups dir for %q: %w", slug, err)
	}

	latest, err := latestBackupTimestamp(backupsDir)
	if err != nil {
		return fmt.Errorf("backup: find latest backup for %q: %w", slug, err)
	}
	if latest == "" {
		return fmt.Errorf("backup: no backups found for profile %q", slug)
	}

	return RestoreAt(slug, latest)
}

// RestoreAt replaces the profile's DB (and vector index if available) with a specific backup.
func RestoreAt(slug, timestamp string) error {
	if timestamp == "" {
		return errors.New("backup: timestamp is required")
	}
	backupsDir, err := config.ProfileBackupsDir(slug)
	if err != nil {
		return fmt.Errorf("backup: get backups dir for %q: %w", slug, err)
	}

	dbSrc := filepath.Join(backupsDir, timestamp+dbSnapshotSuffix)
	dbDst, err := config.ProfileDBPath(slug)
	if err != nil {
		return err
	}
	if err := copyFile(dbSrc, dbDst); err != nil {
		return fmt.Errorf("backup: restore DB for %q: %w", slug, err)
	}

	vecSrc := filepath.Join(backupsDir, timestamp+vecSnapshotSuffix)
	if _, statErr := os.Stat(vecSrc); statErr == nil {
		vecDst, err := config.ProfileVectorPath(slug)
		if err != nil {
			return err
		}
		if err := copyFile(vecSrc, vecDst); err != nil {
			return fmt.Errorf("backup: restore vector for %q: %w", slug, err)
		}
	}

	return nil
}

// ListBackups returns the list of backup timestamps for a profile, newest first.
func ListBackups(slug string) ([]string, error) {
	backupsDir, err := config.ProfileBackupsDir(slug)
	if err != nil {
		return nil, err
	}
	timestamps, err := collectTimestamps(backupsDir)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(timestamps)))
	return timestamps, nil
}

// ---- internal helpers -------------------------------------------------------

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func collectTimestamps(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == dbSnapshotSuffix {
			ts := name[:len(name)-len(dbSnapshotSuffix)]
			seen[ts] = true
		}
	}
	ts := make([]string, 0, len(seen))
	for k := range seen {
		ts = append(ts, k)
	}
	sort.Strings(ts)
	return ts, nil
}

func latestBackupTimestamp(dir string) (string, error) {
	timestamps, err := collectTimestamps(dir)
	if err != nil {
		return "", err
	}
	if len(timestamps) == 0 {
		return "", nil
	}
	return timestamps[len(timestamps)-1], nil
}

func pruneBackups(dir string) error {
	timestamps, err := collectTimestamps(dir)
	if err != nil {
		return err
	}
	if len(timestamps) <= MaxBackups {
		return nil
	}
	// Remove oldest entries beyond MaxBackups.
	toRemove := timestamps[:len(timestamps)-MaxBackups]
	for _, ts := range toRemove {
		for _, ext := range []string{dbSnapshotSuffix, vecSnapshotSuffix} {
			path := filepath.Join(dir, ts+ext)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("backup: prune %s: %w", path, err)
			}
		}
	}
	return nil
}
