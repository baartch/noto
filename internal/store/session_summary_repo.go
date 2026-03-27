package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrSessionSummaryNotFound is returned when a session summary lookup fails.
var ErrSessionSummaryNotFound = errors.New("store: session summary not found")

// SessionSummary is the data model for a compact session handoff context.
type SessionSummary struct {
	ID             string
	ProfileID      string
	ConversationID string
	SummaryText    string
	OpenLoops      string // JSON array of strings
	NextActions    string // JSON array of strings
	CreatedAt      time.Time
}

// SessionSummaryRepo manages CRUD operations for session summaries.
type SessionSummaryRepo struct {
	db *DB
}

// NewSessionSummaryRepo creates a new SessionSummaryRepo.
func NewSessionSummaryRepo(db *DB) *SessionSummaryRepo {
	return &SessionSummaryRepo{db: db}
}

// Create inserts a new session summary.
func (r *SessionSummaryRepo) Create(ctx context.Context, s *SessionSummary) error {
	var convID interface{}
	if s.ConversationID != "" {
		convID = s.ConversationID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO session_summaries
			(id, profile_id, conversation_id, summary_text, open_loops, next_actions)
		VALUES (?, ?, ?, ?, ?, ?)
	`, s.ID, s.ProfileID, convID, s.SummaryText, s.OpenLoops, s.NextActions)
	if err != nil {
		return fmt.Errorf("store: create session summary: %w", err)
	}
	return nil
}

// GetLatestByProfile retrieves the most recent session summary for a profile.
func (r *SessionSummaryRepo) GetLatestByProfile(ctx context.Context, profileID string) (*SessionSummary, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, conversation_id, summary_text, open_loops, next_actions, created_at
		FROM session_summaries
		WHERE profile_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, profileID)

	s := &SessionSummary{}
	var convID sql.NullString
	err := row.Scan(&s.ID, &s.ProfileID, &convID, &s.SummaryText,
		&s.OpenLoops, &s.NextActions, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionSummaryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get latest session summary: %w", err)
	}
	s.ConversationID = convID.String
	return s, nil
}

// ListByProfile returns all session summaries for a profile, newest first.
func (r *SessionSummaryRepo) ListByProfile(ctx context.Context, profileID string) ([]*SessionSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, profile_id, conversation_id, summary_text, open_loops, next_actions, created_at
		FROM session_summaries
		WHERE profile_id = ?
		ORDER BY created_at DESC
	`, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list session summaries: %w", err)
	}
	defer rows.Close()

	var summaries []*SessionSummary
	for rows.Next() {
		s := &SessionSummary{}
		var convID sql.NullString
		if err := rows.Scan(&s.ID, &s.ProfileID, &convID, &s.SummaryText,
			&s.OpenLoops, &s.NextActions, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan session summary: %w", err)
		}
		s.ConversationID = convID.String
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// Delete removes a session summary by ID.
func (r *SessionSummaryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM session_summaries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete session summary: %w", err)
	}
	return nil
}

// DeleteBeforeTime removes summaries for a profile created before the given timestamp.
func (r *SessionSummaryRepo) DeleteBeforeTime(ctx context.Context, profileID string, before time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM session_summaries WHERE profile_id = ? AND created_at < ?`,
		profileID, before)
	return err
}
