package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrProfileNotFound is returned when a profile lookup finds no matching record.
var ErrProfileNotFound = errors.New("store: profile not found")

// ErrProfileNameConflict is returned when a profile name or slug already exists.
var ErrProfileNameConflict = errors.New("store: profile name already exists")

// Profile is the data model for a Noto profile.
type Profile struct {
	ID               string
	Name             string
	Slug             string
	SystemPromptPath string
	DBPath           string
	IsDefault        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ProfileRepo manages CRUD operations for profiles.
type ProfileRepo struct {
	db *DB
}

// NewProfileRepo creates a new ProfileRepo.
func NewProfileRepo(db *DB) *ProfileRepo {
	return &ProfileRepo{db: db}
}

// Create inserts a new profile. Returns ErrProfileNameConflict if name/slug is taken.
func (r *ProfileRepo) Create(ctx context.Context, p *Profile) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profiles (id, name, slug, system_prompt_path, db_path, is_default)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.Slug, p.SystemPromptPath, p.DBPath, boolToInt(p.IsDefault))
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrProfileNameConflict
		}
		return fmt.Errorf("store: create profile: %w", err)
	}
	return nil
}

// GetByID retrieves a profile by its ID.
func (r *ProfileRepo) GetByID(ctx context.Context, id string) (*Profile, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, system_prompt_path, db_path, is_default, created_at, updated_at
		FROM profiles WHERE id = ?
	`, id))
}

// GetBySlug retrieves a profile by its slug.
func (r *ProfileRepo) GetBySlug(ctx context.Context, slug string) (*Profile, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, system_prompt_path, db_path, is_default, created_at, updated_at
		FROM profiles WHERE slug = ?
	`, slug))
}

// GetByName retrieves a profile by its display name.
func (r *ProfileRepo) GetByName(ctx context.Context, name string) (*Profile, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, system_prompt_path, db_path, is_default, created_at, updated_at
		FROM profiles WHERE name = ?
	`, name))
}

// GetDefault retrieves the profile marked as default.
func (r *ProfileRepo) GetDefault(ctx context.Context) (*Profile, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, system_prompt_path, db_path, is_default, created_at, updated_at
		FROM profiles WHERE is_default = 1 LIMIT 1
	`))
}

// GetLastUsed retrieves the most recently updated profile.
func (r *ProfileRepo) GetLastUsed(ctx context.Context) (*Profile, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, system_prompt_path, db_path, is_default, created_at, updated_at
		FROM profiles
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1
	`))
}

// List returns all profiles ordered by name.
func (r *ProfileRepo) List(ctx context.Context) ([]*Profile, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, slug, system_prompt_path, db_path, is_default, created_at, updated_at
		FROM profiles ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("store: list profiles: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var profiles []*Profile
	for rows.Next() {
		p, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// Update updates mutable fields of a profile.
func (r *ProfileRepo) Update(ctx context.Context, p *Profile) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE profiles
		SET name = ?, slug = ?, system_prompt_path = ?, db_path = ?,
		    is_default = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Slug, p.SystemPromptPath, p.DBPath,
		boolToInt(p.IsDefault), time.Now().UTC(), p.ID)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrProfileNameConflict
		}
		return fmt.Errorf("store: update profile: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProfileNotFound
	}
	return nil
}

// SetDefault marks the given profile as default and clears the flag on all others.
func (r *ProfileRepo) SetDefault(ctx context.Context, id string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE profiles SET is_default = 0`); err != nil {
			return fmt.Errorf("store: clear defaults: %w", err)
		}
		result, err := tx.ExecContext(ctx, `UPDATE profiles SET is_default = 1 WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("store: set default: %w", err)
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return ErrProfileNotFound
		}
		return nil
	})
}

// Touch updates the updated_at timestamp for the profile.
func (r *ProfileRepo) Touch(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE profiles
		SET updated_at = ?
		WHERE id = ?
	`, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("store: touch profile: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProfileNotFound
	}
	return nil
}

// Delete removes a profile by ID. Cascades to all child records.
func (r *ProfileRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete profile: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProfileNotFound
	}
	return nil
}

// Count returns the total number of profiles.
func (r *ProfileRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count profiles: %w", err)
	}
	return n, nil
}

// ---- helpers ----------------------------------------------------------------

func (r *ProfileRepo) scanOne(row *sql.Row) (*Profile, error) {
	p := &Profile{}
	var isDefault int
	err := row.Scan(
		&p.ID, &p.Name, &p.Slug, &p.SystemPromptPath, &p.DBPath,
		&isDefault, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan profile: %w", err)
	}
	p.IsDefault = isDefault == 1
	return p, nil
}

func (r *ProfileRepo) scanRow(rows *sql.Rows) (*Profile, error) {
	p := &Profile{}
	var isDefault int
	err := rows.Scan(
		&p.ID, &p.Name, &p.Slug, &p.SystemPromptPath, &p.DBPath,
		&isDefault, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("store: scan profile row: %w", err)
	}
	p.IsDefault = isDefault == 1
	return p, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "UNIQUE constraint failed")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
