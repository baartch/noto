package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrSystemPromptNotFound is returned when no prompt exists for a profile.
var ErrSystemPromptNotFound = errors.New("store: system prompt not found")

// SystemPrompt is the data model for a stored system prompt.
type SystemPrompt struct {
	ID        string
	ProfileID string
	Prompt    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SystemPromptRepo manages CRUD for system prompts.
type SystemPromptRepo struct {
	db *DB
}

func (r *SystemPromptRepo) ensureTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS system_prompts (
			id          TEXT PRIMARY KEY,
			profile_id  TEXT NOT NULL UNIQUE,
			prompt      TEXT NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return fmt.Errorf("store: ensure system prompts table: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_system_prompts_profile_id ON system_prompts(profile_id)
	`)
	if err != nil {
		return fmt.Errorf("store: ensure system prompts index: %w", err)
	}
	return nil
}

// NewSystemPromptRepo creates a SystemPromptRepo.
func NewSystemPromptRepo(db *DB) *SystemPromptRepo {
	return &SystemPromptRepo{db: db}
}

// GetByProfile returns the system prompt for a profile.
func (r *SystemPromptRepo) GetByProfile(ctx context.Context, profileID string) (*SystemPrompt, error) {
	if err := r.ensureTable(ctx); err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, prompt, created_at, updated_at
		FROM system_prompts
		WHERE profile_id = ?
	`, profileID)
	p := &SystemPrompt{}
	if err := row.Scan(&p.ID, &p.ProfileID, &p.Prompt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSystemPromptNotFound
		}
		return nil, fmt.Errorf("store: get system prompt: %w", err)
	}
	return p, nil
}

// Upsert writes the system prompt for a profile.
func (r *SystemPromptRepo) Upsert(ctx context.Context, p *SystemPrompt) error {
	if p == nil {
		return errors.New("store: system prompt is nil")
	}
	if p.ProfileID == "" {
		return errors.New("store: system prompt missing profile_id")
	}
	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO system_prompts
			(id, profile_id, prompt, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			prompt = excluded.prompt,
			updated_at = excluded.updated_at
	`, p.ID, p.ProfileID, p.Prompt, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("store: upsert system prompt: %w", err)
	}
	return nil
}
