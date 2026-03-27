package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrProviderConfigNotFound is returned when no active provider config exists for a profile.
var ErrProviderConfigNotFound = errors.New("store: provider config not found")

// ProviderConfig is the data model for a per-profile provider configuration.
type ProviderConfig struct {
	ID            string
	ProfileID     string
	ProviderType  string
	Endpoint      string
	Model         string
	CredentialRef string // encrypted API key or reference
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ProviderConfigRepo manages CRUD for provider configurations.
type ProviderConfigRepo struct {
	db *DB
}

// NewProviderConfigRepo creates a new ProviderConfigRepo.
func NewProviderConfigRepo(db *DB) *ProviderConfigRepo {
	return &ProviderConfigRepo{db: db}
}

// Upsert inserts or replaces the active provider config for a profile.
func (r *ProviderConfigRepo) Upsert(ctx context.Context, c *ProviderConfig) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO provider_config
			(id, profile_id, provider_type, endpoint, model, credential_ref, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider_type  = excluded.provider_type,
			endpoint       = excluded.endpoint,
			model          = excluded.model,
			credential_ref = excluded.credential_ref,
			is_active      = excluded.is_active,
			updated_at     = datetime('now')
	`, c.ID, c.ProfileID, c.ProviderType, c.Endpoint, c.Model, c.CredentialRef, boolToInt(c.IsActive))
	if err != nil {
		return fmt.Errorf("store: upsert provider config: %w", err)
	}
	return nil
}

// GetActive returns the active provider config for a profile.
func (r *ProviderConfigRepo) GetActive(ctx context.Context, profileID string) (*ProviderConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, provider_type, endpoint, model, credential_ref, is_active,
		       created_at, updated_at
		FROM provider_config
		WHERE profile_id = ? AND is_active = 1
		ORDER BY updated_at DESC
		LIMIT 1
	`, profileID)

	c := &ProviderConfig{}
	var isActive int
	err := row.Scan(&c.ID, &c.ProfileID, &c.ProviderType, &c.Endpoint, &c.Model,
		&c.CredentialRef, &isActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProviderConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get active provider config: %w", err)
	}
	c.IsActive = isActive == 1
	return c, nil
}
