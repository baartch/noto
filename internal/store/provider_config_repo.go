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
	ID             string
	ProfileID      string
	ProviderType   string
	Endpoint       string
	Model          string // default/fallback model set at provider-set time (optional)
	ActiveModel    string // currently selected model (set via /model)
	ExtractorModel string // optional faster model for memory extraction
	CredentialRef  string // encrypted API key
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EffectiveModel returns ActiveModel if set, falling back to Model.
func (c *ProviderConfig) EffectiveModel() string {
	if c.ActiveModel != "" {
		return c.ActiveModel
	}
	return c.Model
}

// EffectiveExtractorModel returns ExtractorModel if set, falling back to EffectiveModel.
func (c *ProviderConfig) EffectiveExtractorModel() string {
	if c.ExtractorModel != "" {
		return c.ExtractorModel
	}
	return c.EffectiveModel()
}

// ProviderConfigRepo manages CRUD for provider configurations.
type ProviderConfigRepo struct {
	db *DB
}

// NewProviderConfigRepo creates a new ProviderConfigRepo.
func NewProviderConfigRepo(db *DB) *ProviderConfigRepo {
	return &ProviderConfigRepo{db: db}
}

// Upsert inserts or replaces a provider config entry.
func (r *ProviderConfigRepo) Upsert(ctx context.Context, c *ProviderConfig) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO provider_config
			(id, profile_id, provider_type, endpoint, model, active_model, extractor_model, credential_ref, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider_type   = excluded.provider_type,
			endpoint        = excluded.endpoint,
			model           = excluded.model,
			active_model    = excluded.active_model,
			extractor_model = excluded.extractor_model,
			credential_ref  = excluded.credential_ref,
			is_active       = excluded.is_active,
			updated_at      = datetime('now')
	`, c.ID, c.ProfileID, c.ProviderType, c.Endpoint, c.Model, c.ActiveModel, c.ExtractorModel,
		c.CredentialRef, boolToInt(c.IsActive))
	if err != nil {
		return fmt.Errorf("store: upsert provider config: %w", err)
	}
	return nil
}

// GetActive returns the active provider config for a profile.
func (r *ProviderConfigRepo) GetActive(ctx context.Context, profileID string) (*ProviderConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, provider_type, endpoint, model, active_model,
		       extractor_model, credential_ref, is_active, created_at, updated_at
		FROM provider_config
		WHERE profile_id = ? AND is_active = 1
		ORDER BY updated_at DESC
		LIMIT 1
	`, profileID)
	return r.scanOne(row)
}

// SetModel updates the active_model for a profile's active provider config.
func (r *ProviderConfigRepo) SetModel(ctx context.Context, profileID, model string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE provider_config SET active_model = ?, updated_at = datetime('now')
		WHERE profile_id = ? AND is_active = 1
	`, model, profileID)
	if err != nil {
		return fmt.Errorf("store: set model: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProviderConfigNotFound
	}
	return nil
}

// SetExtractorModel updates the extractor_model for a profile's active provider config.
func (r *ProviderConfigRepo) SetExtractorModel(ctx context.Context, profileID, model string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE provider_config SET extractor_model = ?, updated_at = datetime('now')
		WHERE profile_id = ? AND is_active = 1
	`, model, profileID)
	if err != nil {
		return fmt.Errorf("store: set extractor model: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProviderConfigNotFound
	}
	return nil
}

func (r *ProviderConfigRepo) scanOne(row *sql.Row) (*ProviderConfig, error) {
	c := &ProviderConfig{}
	var isActive int
	err := row.Scan(&c.ID, &c.ProfileID, &c.ProviderType, &c.Endpoint, &c.Model, &c.ActiveModel,
		&c.ExtractorModel, &c.CredentialRef, &isActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProviderConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan provider config: %w", err)
	}
	c.IsActive = isActive == 1
	return c, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
