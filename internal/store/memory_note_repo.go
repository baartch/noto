package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrMemoryNoteNotFound is returned when a memory note lookup fails.
var ErrMemoryNoteNotFound = errors.New("store: memory note not found")

// MemoryCategory classifies the type of knowledge captured in a note.
type MemoryCategory string

const (
	CategoryFact       MemoryCategory = "fact"
	CategoryProgress   MemoryCategory = "progress"
	CategoryBlocker    MemoryCategory = "blocker"
	CategoryActionItem MemoryCategory = "action_item"
	CategoryOther      MemoryCategory = "other"
)

// MemoryNote is the data model for a durable continuity knowledge note.
type MemoryNote struct {
	ID               string
	ProfileID        string
	ConversationID   string // empty string means NULL in the DB
	Category         MemoryCategory
	Content          string
	Importance       int    // 1–10
	SourceMessageIDs string // JSON array
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// MemoryNoteRepo manages CRUD operations for memory notes.
type MemoryNoteRepo struct {
	db *DB
}

// NewMemoryNoteRepo creates a new MemoryNoteRepo.
func NewMemoryNoteRepo(db *DB) *MemoryNoteRepo {
	return &MemoryNoteRepo{db: db}
}

// Create inserts a new memory note.
func (r *MemoryNoteRepo) Create(ctx context.Context, n *MemoryNote) error {
	var convID interface{}
	if n.ConversationID != "" {
		convID = n.ConversationID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO memory_notes
			(id, profile_id, conversation_id, category, content, importance, source_message_ids)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, n.ID, n.ProfileID, convID, string(n.Category),
		n.Content, n.Importance, n.SourceMessageIDs)
	if err != nil {
		return fmt.Errorf("store: create memory note: %w", err)
	}
	return nil
}

// GetByID retrieves a memory note by its ID.
func (r *MemoryNoteRepo) GetByID(ctx context.Context, id string) (*MemoryNote, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, COALESCE(conversation_id,''), category, content, importance,
		       source_message_ids, created_at, updated_at
		FROM memory_notes WHERE id = ?
	`, id)
	n := &MemoryNote{}
	var cat string
	err := row.Scan(
		&n.ID, &n.ProfileID, &n.ConversationID, &cat, &n.Content, &n.Importance,
		&n.SourceMessageIDs, &n.CreatedAt, &n.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMemoryNoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get memory note: %w", err)
	}
	n.Category = MemoryCategory(cat)
	return n, nil
}

// ListByProfile returns all memory notes for a profile, ordered by importance desc, then created_at desc.
func (r *MemoryNoteRepo) ListByProfile(ctx context.Context, profileID string) ([]*MemoryNote, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, profile_id, COALESCE(conversation_id,''), category, content, importance,
		       source_message_ids, created_at, updated_at
		FROM memory_notes
		WHERE profile_id = ?
		ORDER BY importance DESC, created_at DESC
	`, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list memory notes: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var notes []*MemoryNote
	for rows.Next() {
		n := &MemoryNote{}
		var cat string
		if err := rows.Scan(
			&n.ID, &n.ProfileID, &n.ConversationID, &cat, &n.Content, &n.Importance,
			&n.SourceMessageIDs, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("store: scan memory note: %w", err)
		}
		n.Category = MemoryCategory(cat)
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

// Update updates the content and importance of a memory note.
func (r *MemoryNoteRepo) Update(ctx context.Context, n *MemoryNote) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE memory_notes
		SET content = ?, importance = ?, category = ?, source_message_ids = ?, updated_at = ?
		WHERE id = ? AND profile_id = ?
	`, n.Content, n.Importance, string(n.Category), n.SourceMessageIDs,
		time.Now().UTC(), n.ID, n.ProfileID)
	if err != nil {
		return fmt.Errorf("store: update memory note: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrMemoryNoteNotFound
	}
	return nil
}

// Delete removes a memory note.
func (r *MemoryNoteRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM memory_notes WHERE id = ?`, id)
	return err
}
