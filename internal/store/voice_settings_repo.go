package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidVoiceProvider is returned when a voice backend is not supported.
var (
	ErrInvalidVoiceProvider  = errors.New("store: invalid voice provider")
	ErrInvalidVoiceSelection = errors.New("store: invalid voice selection")
)

// VoiceSettings stores speech-to-text selection for one profile.
type VoiceSettings struct {
	Provider string
	Model    string
}

// VoiceSettingsRepo reads and writes settings in a profile-local database.
type VoiceSettingsRepo struct {
	db *DB
}

// NewVoiceSettingsRepo creates a profile-local voice settings repository.
func NewVoiceSettingsRepo(db *DB) *VoiceSettingsRepo {
	return &VoiceSettingsRepo{db: db}
}

// Get returns the voice settings for the profile.
func (r *VoiceSettingsRepo) Get(ctx context.Context) (VoiceSettings, error) {
	var settings VoiceSettings
	err := r.db.QueryRowContext(ctx, `
		SELECT provider, model
		FROM voice_settings
		WHERE singleton = 1
	`).Scan(&settings.Provider, &settings.Model)
	if err != nil {
		return VoiceSettings{}, fmt.Errorf("store: get voice settings: %w", err)
	}
	return settings, nil
}

// Save replaces this profile's voice selection.
func (r *VoiceSettingsRepo) Save(ctx context.Context, settings VoiceSettings) error {
	switch settings.Provider {
	case "":
		if settings.Model != "" {
			return fmt.Errorf("%w: model requires a provider", ErrInvalidVoiceSelection)
		}
	case "local", "provider":
		if settings.Model == "" || strings.TrimSpace(settings.Model) != settings.Model {
			return fmt.Errorf("%w: model is required", ErrInvalidVoiceSelection)
		}
	default:
		return fmt.Errorf("%w: %q", ErrInvalidVoiceProvider, settings.Provider)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO voice_settings (singleton, provider, model)
		VALUES (1, ?, ?)
		ON CONFLICT(singleton) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			updated_at = datetime('now')
	`, settings.Provider, settings.Model)
	if err != nil {
		return fmt.Errorf("store: save voice settings: %w", err)
	}
	return nil
}
