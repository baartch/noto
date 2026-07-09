package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// MonthlySummaryAllRemaining is the sentinel value representing unbounded
// monthly summary coverage for older history.
const MonthlySummaryAllRemaining = -1

// ErrTimelineSettingsNotFound is returned when a profile has no stored timeline settings row.
var ErrTimelineSettingsNotFound = errors.New("store: timeline settings not found")

// TimelineSettings persists profile-local context assembly settings.
type TimelineSettings struct {
	ProfileID            string
	RawNoteDays          int
	WeeklySummaryWeeks   int
	MonthlySummaryMonths int // -1 means all remaining
	DedupMaxAgeDays      int // notes older than this are never updated/merged by dedup
	UpdatedAt            time.Time
}

// Validate checks that timeline settings satisfy persisted input rules.
func (s *TimelineSettings) Validate() error {
	if s == nil {
		return errors.New("store: timeline settings is nil")
	}
	if s.RawNoteDays <= 0 {
		return errors.New("store: raw_note_days must be > 0")
	}
	if s.WeeklySummaryWeeks <= 0 {
		return errors.New("store: weekly_summary_weeks must be > 0")
	}
	if s.MonthlySummaryMonths != MonthlySummaryAllRemaining && s.MonthlySummaryMonths <= 0 {
		return errors.New("store: monthly_summary_months must be > 0 or all_remaining")
	}
	if s.DedupMaxAgeDays <= 0 {
		return errors.New("store: dedup_max_age_days must be > 0")
	}
	return nil
}

// Normalize returns the settings value or defaults when nil.
func (s *TimelineSettings) Normalize() *TimelineSettings {
	if s == nil {
		return DefaultTimelineSettings("")
	}
	return s
}

// DefaultTimelineSettings returns the default timeline settings for a profile.
func DefaultTimelineSettings(profileID string) *TimelineSettings {
	return &TimelineSettings{
		ProfileID:            profileID,
		RawNoteDays:          30,
		WeeklySummaryWeeks:   8,
		MonthlySummaryMonths: MonthlySummaryAllRemaining,
		DedupMaxAgeDays:      7,
	}
}

// TimelineSettingsRepo manages persistence for timeline settings.
type TimelineSettingsRepo struct {
	db *DB
}

// NewTimelineSettingsRepo creates a repository for profile timeline settings.
func NewTimelineSettingsRepo(db *DB) *TimelineSettingsRepo {
	return &TimelineSettingsRepo{db: db}
}

// Get loads persisted timeline settings for a profile.
func (r *TimelineSettingsRepo) Get(ctx context.Context, profileID string) (*TimelineSettings, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT key, value, updated_at
		FROM settings
		WHERE profile_id = ? AND key IN ('raw_note_days', 'weekly_summary_weeks', 'monthly_summary_months', 'dedup_max_age_days')
	`, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: get timeline settings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	s := DefaultTimelineSettings(profileID)
	found := false
	for rows.Next() {
		var key, value string
		var updatedAt time.Time
		if err := rows.Scan(&key, &value, &updatedAt); err != nil {
			return nil, fmt.Errorf("store: scan timeline settings: %w", err)
		}
		found = true
		s.UpdatedAt = updatedAt
		switch key {
		case "raw_note_days":
			if _, err := fmt.Sscanf(value, "%d", &s.RawNoteDays); err != nil {
				return nil, fmt.Errorf("store: parse raw_note_days: %w", err)
			}
		case "weekly_summary_weeks":
			if _, err := fmt.Sscanf(value, "%d", &s.WeeklySummaryWeeks); err != nil {
				return nil, fmt.Errorf("store: parse weekly_summary_weeks: %w", err)
			}
		case "monthly_summary_months":
			if value == "all_remaining" {
				s.MonthlySummaryMonths = MonthlySummaryAllRemaining
			} else if _, err := fmt.Sscanf(value, "%d", &s.MonthlySummaryMonths); err != nil {
				return nil, fmt.Errorf("store: parse monthly_summary_months: %w", err)
			}
		case "dedup_max_age_days":
			if _, err := fmt.Sscanf(value, "%d", &s.DedupMaxAgeDays); err != nil {
				return nil, fmt.Errorf("store: parse dedup_max_age_days: %w", err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate timeline settings: %w", err)
	}
	if !found {
		return nil, ErrTimelineSettingsNotFound
	}
	return s.Normalize(), nil
}

// GetOrDefault loads timeline settings or returns defaults when none exist.
func (r *TimelineSettingsRepo) GetOrDefault(ctx context.Context, profileID string) (*TimelineSettings, error) {
	s, err := r.Get(ctx, profileID)
	if err == nil {
		return s, nil
	}
	if !errors.Is(err, ErrTimelineSettingsNotFound) {
		return nil, err
	}
	return DefaultTimelineSettings(profileID), nil
}

// Upsert persists timeline settings for a profile.
func (r *TimelineSettingsRepo) Upsert(ctx context.Context, s *TimelineSettings) error {
	if s == nil {
		return errors.New("store: timeline settings is nil")
	}
	s = s.Normalize()
	if err := s.Validate(); err != nil {
		return err
	}
	monthlyValue := strconv.Itoa(s.MonthlySummaryMonths)
	if s.MonthlySummaryMonths == MonthlySummaryAllRemaining {
		monthlyValue = "all_remaining"
	}
	entries := []struct {
		key   string
		value string
	}{
		{key: "raw_note_days", value: strconv.Itoa(s.RawNoteDays)},
		{key: "weekly_summary_weeks", value: strconv.Itoa(s.WeeklySummaryWeeks)},
		{key: "monthly_summary_months", value: monthlyValue},
		{key: "dedup_max_age_days", value: strconv.Itoa(s.DedupMaxAgeDays)},
	}
	for _, entry := range entries {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO settings (profile_id, key, value, updated_at)
			VALUES (?, ?, ?, datetime('now'))
			ON CONFLICT(profile_id, key) DO UPDATE SET
				value = excluded.value,
				updated_at = datetime('now')
		`, s.ProfileID, entry.key, entry.value); err != nil {
			return fmt.Errorf("store: upsert timeline setting %s: %w", entry.key, err)
		}
	}
	return nil
}
