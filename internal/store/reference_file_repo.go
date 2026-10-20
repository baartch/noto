package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ReferenceFileStatus identifies a reference catalog row's indexing state.
type ReferenceFileStatus string

const (
	// ReferenceFilePending means the source changed and its description is being refreshed.
	ReferenceFilePending ReferenceFileStatus = "pending"
	// ReferenceFileReady means the stored description matches the source fingerprint.
	ReferenceFileReady ReferenceFileStatus = "ready"
	// ReferenceFileFailed means the source could not be read or described.
	ReferenceFileFailed ReferenceFileStatus = "failed"
)

// ReferenceFile is catalog metadata only; source text is never persisted here.
type ReferenceFile struct {
	ID, ProfileID, RelativePath, MediaType   string
	Fingerprint, SummaryFingerprint, Summary string
	Status                                   ReferenceFileStatus
	LastError                                string
	Size, ModTimeNS                          int64
	IndexedAt                                *time.Time
}

// ReferenceFileRepo manages the profile-local reference-file catalog.
type ReferenceFileRepo struct{ db *DB }

// NewReferenceFileRepo creates a repository for reference-file metadata.
func NewReferenceFileRepo(db *DB) *ReferenceFileRepo { return &ReferenceFileRepo{db: db} }

// List returns a profile's catalog in deterministic relative-path order.
func (r *ReferenceFileRepo) List(ctx context.Context, profileID string) ([]ReferenceFile, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, profile_id, relative_path, media_type, fingerprint, summary_fingerprint,
		       summary, status, size_bytes, mod_time_ns, last_error, indexed_at
		FROM reference_files WHERE profile_id = ? ORDER BY relative_path ASC
	`, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list reference files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var files []ReferenceFile
	for rows.Next() {
		file, err := scanReferenceFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

// DeletePath removes a confirmed-missing path from one profile catalog.
func (r *ReferenceFileRepo) DeletePath(ctx context.Context, profileID, relativePath string) error {
	if profileID == "" || relativePath == "" {
		return errors.New("store: reference file profile and path are required")
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM reference_files WHERE profile_id = ? AND relative_path = ?`, profileID, relativePath); err != nil {
		return fmt.Errorf("store: delete reference file path: %w", err)
	}
	return nil
}

// Upsert inserts metadata or updates the row for the same profile-relative path.
func (r *ReferenceFileRepo) Upsert(ctx context.Context, file ReferenceFile) error {
	if file.ID == "" || file.ProfileID == "" || file.RelativePath == "" {
		return errors.New("store: reference file requires id, profile, and relative path")
	}
	if file.Status != ReferenceFilePending && file.Status != ReferenceFileReady && file.Status != ReferenceFileFailed {
		return fmt.Errorf("store: invalid reference file status %q", file.Status)
	}
	if file.Size < 0 {
		return errors.New("store: reference file size cannot be negative")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO reference_files (
			id, profile_id, relative_path, media_type, fingerprint, summary_fingerprint,
			summary, status, size_bytes, mod_time_ns, last_error, indexed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, relative_path) DO UPDATE SET
			media_type=excluded.media_type,
			fingerprint=excluded.fingerprint,
			summary_fingerprint=excluded.summary_fingerprint,
			summary=excluded.summary,
			status=excluded.status,
			size_bytes=excluded.size_bytes,
			mod_time_ns=excluded.mod_time_ns,
			last_error=excluded.last_error,
			indexed_at=excluded.indexed_at
	`, file.ID, file.ProfileID, file.RelativePath, file.MediaType, file.Fingerprint,
		file.SummaryFingerprint, file.Summary, string(file.Status), file.Size,
		file.ModTimeNS, file.LastError, file.IndexedAt)
	if err != nil {
		return fmt.Errorf("store: upsert reference file: %w", err)
	}
	return nil
}

// DeleteMissing removes paths absent from a completed enumeration atomically.
func (r *ReferenceFileRepo) DeleteMissing(ctx context.Context, profileID string, seenPaths []string) (int64, error) {
	if profileID == "" {
		return 0, errors.New("store: reference file profile is required")
	}
	seen := append([]string(nil), seenPaths...)
	sort.Strings(seen)
	seen = compactStrings(seen)
	var deleted int64
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT relative_path FROM reference_files WHERE profile_id = ?`, profileID)
		if err != nil {
			return fmt.Errorf("store: enumerate reference files for reconcile: %w", err)
		}
		var existing []string
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				_ = rows.Close()
				return err
			}
			existing = append(existing, path)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, path := range existing {
			idx := sort.SearchStrings(seen, path)
			if idx < len(seen) && seen[idx] == path {
				continue
			}
			result, err := tx.ExecContext(ctx, `DELETE FROM reference_files WHERE profile_id = ? AND relative_path = ?`, profileID, path)
			if err != nil {
				return fmt.Errorf("store: delete missing reference file: %w", err)
			}
			n, _ := result.RowsAffected()
			deleted += n
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func scanReferenceFile(row interface{ Scan(...any) error }) (ReferenceFile, error) {
	var file ReferenceFile
	var status string
	var indexed sql.NullTime
	if err := row.Scan(&file.ID, &file.ProfileID, &file.RelativePath, &file.MediaType,
		&file.Fingerprint, &file.SummaryFingerprint, &file.Summary, &status,
		&file.Size, &file.ModTimeNS, &file.LastError, &indexed); err != nil {
		return ReferenceFile{}, fmt.Errorf("store: scan reference file: %w", err)
	}
	file.Status = ReferenceFileStatus(status)
	if indexed.Valid {
		t := indexed.Time
		file.IndexedAt = &t
	}
	return file, nil
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
