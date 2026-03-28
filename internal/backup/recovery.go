package backup

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"

	_ "modernc.org/sqlite"

	"noto/internal/config"
)

// RecoveryResult describes what the recovery coordinator did.
type RecoveryResult struct {
	// Action is one of "none", "auto_repaired", "backup_restored", "failed".
	Action  string
	Details string
}

// Recover attempts to recover a profile's database.
// It first validates the DB via SQLite's integrity check, then attempts an
// auto-repair (VACUUM/REINDEX) if corrupted, and finally falls back to restoring
// the most recent backup if repair fails.
func Recover(slug string, w io.Writer) RecoveryResult {
	dbPath, err := config.ProfileDBPath(slug)
	if err != nil {
		return RecoveryResult{Action: "failed", Details: err.Error()}
	}

	// Quick existence check.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// DB missing — restore from backup.
		fmt.Fprintln(w, "noto: database missing, attempting backup restore...")
		if err := Restore(slug); err != nil {
			return RecoveryResult{Action: "failed", Details: fmt.Sprintf("backup restore failed: %v", err)}
		}
		return RecoveryResult{Action: "backup_restored", Details: "restored from latest backup"}
	}

	ok, details, err := integrityCheck(dbPath)
	if err != nil {
		return RecoveryResult{Action: "failed", Details: fmt.Sprintf("integrity check failed: %v", err)}
	}
	if ok {
		return RecoveryResult{Action: "none", Details: "database appears healthy"}
	}

	fmt.Fprintf(w, "noto: database integrity failed (%s). attempting auto-repair...\n", details)
	if repaired, repairErr := attemptRepair(dbPath); repairErr == nil && repaired {
		return RecoveryResult{Action: "auto_repaired", Details: "auto-repair succeeded"}
	} else if repairErr != nil {
		fmt.Fprintf(w, "noto: auto-repair failed: %v\n", repairErr)
	}

	fmt.Fprintln(w, "noto: attempting backup restore...")
	if err := Restore(slug); err != nil {
		return RecoveryResult{Action: "failed", Details: fmt.Sprintf("backup restore failed: %v", err)}
	}
	return RecoveryResult{Action: "backup_restored", Details: "restored from latest backup"}
}

func integrityCheck(path string) (bool, string, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false, "", err
	}
	defer db.Close()

	rows, err := db.Query(`PRAGMA integrity_check;`)
	if err != nil {
		return false, "", err
	}
	defer rows.Close()

	var issues []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return false, "", err
		}
		if line != "ok" {
			issues = append(issues, line)
		}
	}
	if err := rows.Err(); err != nil {
		return false, "", err
	}
	if len(issues) == 0 {
		return true, "", nil
	}
	return false, strings.Join(issues, "; "), nil
}

func attemptRepair(path string) (bool, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false, err
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
		return false, fmt.Errorf("checkpoint: %w", err)
	}
	if _, err := db.Exec(`REINDEX;`); err != nil {
		return false, fmt.Errorf("reindex: %w", err)
	}
	if _, err := db.Exec(`VACUUM;`); err != nil {
		return false, fmt.Errorf("vacuum: %w", err)
	}

	ok, _, err := integrityCheck(path)
	if err != nil {
		return false, err
	}
	return ok, nil
}
