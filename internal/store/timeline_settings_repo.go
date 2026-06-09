package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const MonthlySummaryAllRemaining = -1

// ErrTimelineSettingsNotFound is returned when a profile has no stored timeline settings row.
var ErrTimelineSettingsNotFound = errors.New("store: timeline settings not found")

// TimelineSettings persists profile-local context assembly settings.
type TimelineSettings struct {
	ProfileID             string
	RawNoteDays           int
	WeeklySummaryWeeks    int
	MonthlySummaryMonths  int // -1 means all remaining
	UpdatedAt             time.Time
}

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
	return nil
}

func (s *TimelineSettings) Normalize() *TimelineSettings {
	if s == nil {
		return DefaultTimelineSettings("")
	}
	if s.RawNoteDays <= 0 {
		s.RawNoteDays = 30
	}
	if s.WeeklySummaryWeeks <= 0 {
		s.WeeklySummaryWeeks = 8
	}
	if s.MonthlySummaryMonths == 0 {
		s.MonthlySummaryMonths = MonthlySummaryAllRemaining
	}
	return s
}

func DefaultTimelineSettings(profileID string) *TimelineSettings {
	return &TimelineSettings{
		ProfileID:            profileID,
		RawNoteDays:          30,
		WeeklySummaryWeeks:   8,
		MonthlySummaryMonths: MonthlySummaryAllRemaining,
	}
}

// TimelineSettingsRepo manages persistence for timeline settings.
type TimelineSettingsRepo struct {
	db *DB
}

func NewTimelineSettingsRepo(db *DB) *TimelineSettingsRepo {
	return &TimelineSettingsRepo{db: db}
}

func (r *TimelineSettingsRepo) Get(ctx context.Context, profileID string) (*TimelineSettings, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT key, value, updated_at
		FROM settings
		WHERE profile_id = ? AND key IN ('raw_note_days', 'weekly_summary_weeks', 'monthly_summary_months')
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

func (r *TimelineSettingsRepo) Upsert(ctx context.Context, s *TimelineSettings) error {
	if s == nil {
		return errors.New("store: timeline settings is nil")
	}
	s = s.Normalize()
	if err := s.Validate(); err != nil {
		return err
	}
	monthlyValue := fmt.Sprintf("%d", s.MonthlySummaryMonths)
	if s.MonthlySummaryMonths == MonthlySummaryAllRemaining {
		monthlyValue = "all_remaining"
	}
	entries := []struct {
		key   string
		value string
	}{
		{key: "raw_note_days", value: fmt.Sprintf("%d", s.RawNoteDays)},
		{key: "weekly_summary_weeks", value: fmt.Sprintf("%d", s.WeeklySummaryWeeks)},
		{key: "monthly_summary_months", value: monthlyValue},
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
