package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrMemorySummaryNotFound is returned when a summary artifact cannot be found.
var ErrMemorySummaryNotFound = errors.New("store: memory summary not found")

// Summary artifact and freshness constants.
const (
	SummaryTypeWeekly  = "weekly"
	SummaryTypeMonthly = "monthly"

	SummaryFresh        = "fresh"
	SummaryStale        = "stale"
	SummaryRegenerating = "regenerating"
)

// MemorySummary stores either a weekly or monthly rollup artifact.
type MemorySummary struct {
	ID                 string
	ProfileID          string
	SummaryType        string
	PeriodKey          string
	PeriodStart        time.Time
	PeriodEnd          time.Time
	Content            string
	SourceStateVersion string
	FreshnessState     string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// MemorySummaryRepo manages weekly/monthly summary artifacts.
type MemorySummaryRepo struct {
	db *DB
}

// NewMemorySummaryRepo creates a repository for weekly and monthly summaries.
func NewMemorySummaryRepo(db *DB) *MemorySummaryRepo {
	return &MemorySummaryRepo{db: db}
}

// Upsert creates or replaces a weekly/monthly summary artifact for its period.
func (r *MemorySummaryRepo) Upsert(ctx context.Context, s *MemorySummary) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO memory_summaries
			(id, profile_id, summary_type, period_key, period_start, period_end, content, source_state_version, freshness_state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, summary_type, period_key) DO UPDATE SET
			period_start = excluded.period_start,
			period_end = excluded.period_end,
			content = excluded.content,
			source_state_version = excluded.source_state_version,
			freshness_state = excluded.freshness_state,
			updated_at = excluded.updated_at
	`, s.ID, s.ProfileID, s.SummaryType, s.PeriodKey, s.PeriodStart, s.PeriodEnd, s.Content, s.SourceStateVersion, s.FreshnessState, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: upsert memory summary: %w", err)
	}
	return nil
}

// GetByPeriod loads a summary artifact by profile, type, and period key.
func (r *MemorySummaryRepo) GetByPeriod(ctx context.Context, profileID, summaryType, periodKey string) (*MemorySummary, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, summary_type, period_key, period_start, period_end, content, source_state_version, freshness_state, created_at, updated_at
		FROM memory_summaries
		WHERE profile_id = ? AND summary_type = ? AND period_key = ?
	`, profileID, summaryType, periodKey)
	var s MemorySummary
	if err := row.Scan(&s.ID, &s.ProfileID, &s.SummaryType, &s.PeriodKey, &s.PeriodStart, &s.PeriodEnd, &s.Content, &s.SourceStateVersion, &s.FreshnessState, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMemorySummaryNotFound
		}
		return nil, fmt.Errorf("store: get memory summary: %w", err)
	}
	return &s, nil
}

// ListByProfileAndType lists summary artifacts for a profile and summary type.
func (r *MemorySummaryRepo) ListByProfileAndType(ctx context.Context, profileID, summaryType string) ([]*MemorySummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, profile_id, summary_type, period_key, period_start, period_end, content, source_state_version, freshness_state, created_at, updated_at
		FROM memory_summaries
		WHERE profile_id = ? AND summary_type = ?
		ORDER BY period_start DESC
	`, profileID, summaryType)
	if err != nil {
		return nil, fmt.Errorf("store: list memory summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*MemorySummary
	for rows.Next() {
		var s MemorySummary
		if err := rows.Scan(&s.ID, &s.ProfileID, &s.SummaryType, &s.PeriodKey, &s.PeriodStart, &s.PeriodEnd, &s.Content, &s.SourceStateVersion, &s.FreshnessState, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan memory summary: %w", err)
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// MarkFreshness updates the freshness state for a stored summary artifact.
func (r *MemorySummaryRepo) MarkFreshness(ctx context.Context, profileID, summaryType, periodKey, freshness string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE memory_summaries
		SET freshness_state = ?, updated_at = datetime('now')
		WHERE profile_id = ? AND summary_type = ? AND period_key = ?
	`, freshness, profileID, summaryType, periodKey)
	if err != nil {
		return fmt.Errorf("store: mark summary freshness: %w", err)
	}
	return nil
}
