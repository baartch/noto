package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrManifestNotFound is returned when no manifest exists for a profile.
var ErrManifestNotFound = errors.New("store: vector manifest not found")

// VectorManifestStatus describes the health of a profile's vector index.
type VectorManifestStatus string

const (
	VectorManifestReady      VectorManifestStatus = "ready"
	VectorManifestStale      VectorManifestStatus = "stale"
	VectorManifestRebuilding VectorManifestStatus = "rebuilding"
	VectorManifestFailed     VectorManifestStatus = "failed"
)

// VectorManifest holds per-profile vector index metadata.
type VectorManifest struct {
	ID                 string
	ProfileID          string
	IndexPath          string
	IndexFormatVersion string
	EmbeddingModel     string
	EmbeddingDim       int
	SourceStateVersion string
	Status             VectorManifestStatus
}

// VectorEntry represents a single record tracked in the vector index manifest.
type VectorEntry struct {
	ID             string
	ProfileID      string
	SourceType     string // "memory_note" | "session_summary" | "message"
	SourceID       string
	ChunkHash      string
	EmbeddingModel string
	EmbeddingDim   int
	VectorRef      string
}

// VectorManifestRepo manages the vector_index_manifest and vector_index_entries tables.
type VectorManifestRepo struct {
	db *DB
}

// NewVectorManifestRepo creates a new VectorManifestRepo.
func NewVectorManifestRepo(db *DB) *VectorManifestRepo {
	return &VectorManifestRepo{db: db}
}

// GetManifest retrieves the vector index manifest for a profile.
func (r *VectorManifestRepo) GetManifest(ctx context.Context, profileID string) (*VectorManifest, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, index_path, index_format_version, embedding_model,
		       embedding_dim, source_state_version, status
		FROM vector_index_manifest
		WHERE profile_id = ?
	`, profileID)

	var m VectorManifest
	var status string
	err := row.Scan(
		&m.ID, &m.ProfileID, &m.IndexPath, &m.IndexFormatVersion,
		&m.EmbeddingModel, &m.EmbeddingDim, &m.SourceStateVersion, &status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrManifestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get vector manifest: %w", err)
	}
	m.Status = VectorManifestStatus(status)
	return &m, nil
}

// UpsertManifest inserts or replaces the vector index manifest for a profile.
func (r *VectorManifestRepo) UpsertManifest(ctx context.Context, m *VectorManifest) error {
	if m.ProfileID == "" {
		return fmt.Errorf("store: manifest missing profile_id")
	}
	if m.IndexPath == "" {
		return fmt.Errorf("store: manifest missing index_path")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO vector_index_manifest
			(id, profile_id, index_path, index_format_version, embedding_model,
			 embedding_dim, source_state_version, status, last_sync_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			index_path           = excluded.index_path,
			index_format_version = excluded.index_format_version,
			embedding_model      = excluded.embedding_model,
			embedding_dim        = excluded.embedding_dim,
			source_state_version = excluded.source_state_version,
			status               = excluded.status,
			last_sync_at         = excluded.last_sync_at
	`,
		m.ID, m.ProfileID, m.IndexPath, m.IndexFormatVersion, m.EmbeddingModel,
		m.EmbeddingDim, m.SourceStateVersion, string(m.Status), time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("store: upsert vector manifest: %w", err)
	}
	return nil
}

// SetManifestStatusStr updates the manifest status using a plain string value.
// This satisfies the vector.ManifestSetter / vector.ManifestStatusSetter interfaces.
func (r *VectorManifestRepo) SetManifestStatusStr(ctx context.Context, profileID string, status string) error {
	return r.SetManifestStatus(ctx, profileID, VectorManifestStatus(status))
}

// SetManifestStatus updates only the status field of the manifest for a profile.
func (r *VectorManifestRepo) SetManifestStatus(ctx context.Context, profileID string, status VectorManifestStatus) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE vector_index_manifest SET status = ? WHERE profile_id = ?
	`, string(status), profileID)
	if err != nil {
		return fmt.Errorf("store: set manifest status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrManifestNotFound
	}
	return nil
}

// UpsertEntry inserts or updates a vector index entry.
func (r *VectorManifestRepo) UpsertEntry(ctx context.Context, e *VectorEntry) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO vector_index_entries
			(id, profile_id, source_type, source_id, chunk_hash, embedding_model,
			 embedding_dim, vector_ref, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, source_type, source_id, chunk_hash) DO UPDATE SET
			embedding_model = excluded.embedding_model,
			embedding_dim   = excluded.embedding_dim,
			vector_ref      = excluded.vector_ref,
			updated_at      = excluded.updated_at
	`,
		e.ID, e.ProfileID, e.SourceType, e.SourceID, e.ChunkHash,
		e.EmbeddingModel, e.EmbeddingDim, e.VectorRef, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("store: upsert vector entry: %w", err)
	}
	return nil
}

// DeleteEntry removes the vector entry for a given source.
func (r *VectorManifestRepo) DeleteEntry(ctx context.Context, profileID, sourceType, sourceID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM vector_index_entries
		WHERE profile_id = ? AND source_type = ? AND source_id = ?
	`, profileID, sourceType, sourceID)
	if err != nil {
		return fmt.Errorf("store: delete vector entry: %w", err)
	}
	return nil
}

// ListEntries returns all vector index entries for a profile.
func (r *VectorManifestRepo) ListEntries(ctx context.Context, profileID string) ([]*VectorEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, profile_id, source_type, source_id, chunk_hash,
		       embedding_model, embedding_dim, vector_ref
		FROM vector_index_entries
		WHERE profile_id = ?
		ORDER BY updated_at ASC
	`, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list vector entries: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var entries []*VectorEntry
	for rows.Next() {
		var e VectorEntry
		if err := rows.Scan(
			&e.ID, &e.ProfileID, &e.SourceType, &e.SourceID, &e.ChunkHash,
			&e.EmbeddingModel, &e.EmbeddingDim, &e.VectorRef,
		); err != nil {
			return nil, fmt.Errorf("store: scan vector entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}
