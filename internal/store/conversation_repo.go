package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrConversationNotFound is returned when no matching conversation is found.
var ErrConversationNotFound = errors.New("store: conversation not found")

// ConversationStatus represents the lifecycle state of a conversation.
type ConversationStatus string

const (
	ConversationActive   ConversationStatus = "active"
	ConversationArchived ConversationStatus = "archived"
)

// Conversation is the data model for a chat session.
type Conversation struct {
	ID        string
	ProfileID string
	StartedAt time.Time
	EndedAt   *time.Time
	Status    ConversationStatus
}

// ConversationRepo manages CRUD operations for conversations.
type ConversationRepo struct {
	db *DB
}

// NewConversationRepo creates a new ConversationRepo.
func NewConversationRepo(db *DB) *ConversationRepo {
	return &ConversationRepo{db: db}
}

// Create inserts a new conversation.
func (r *ConversationRepo) Create(ctx context.Context, c *Conversation) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO conversations (id, profile_id, status)
		VALUES (?, ?, ?)
	`, c.ID, c.ProfileID, string(c.Status))
	if err != nil {
		return fmt.Errorf("store: create conversation: %w", err)
	}
	return nil
}

// GetByID retrieves a conversation by its ID.
func (r *ConversationRepo) GetByID(ctx context.Context, id string) (*Conversation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, profile_id, started_at, ended_at, status
		FROM conversations WHERE id = ?
	`, id)
	return r.scanOne(row)
}

// ListByProfile returns all conversations for a profile, newest first.
func (r *ConversationRepo) ListByProfile(ctx context.Context, profileID string) ([]*Conversation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, profile_id, started_at, ended_at, status
		FROM conversations WHERE profile_id = ?
		ORDER BY started_at DESC
	`, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list conversations: %w", err)
	}
	defer rows.Close()

	var convs []*Conversation
	for rows.Next() {
		c, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

// Archive sets the conversation status to archived and records its end time.
func (r *ConversationRepo) Archive(ctx context.Context, id string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE conversations SET status = 'archived', ended_at = ? WHERE id = ?
	`, now, id)
	if err != nil {
		return fmt.Errorf("store: archive conversation: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrConversationNotFound
	}
	return nil
}

// ---- helpers ----------------------------------------------------------------

func (r *ConversationRepo) scanOne(row *sql.Row) (*Conversation, error) {
	c := &Conversation{}
	var status string
	err := row.Scan(&c.ID, &c.ProfileID, &c.StartedAt, &c.EndedAt, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan conversation: %w", err)
	}
	c.Status = ConversationStatus(status)
	return c, nil
}

func (r *ConversationRepo) scanRow(rows *sql.Rows) (*Conversation, error) {
	c := &Conversation{}
	var status string
	err := rows.Scan(&c.ID, &c.ProfileID, &c.StartedAt, &c.EndedAt, &status)
	if err != nil {
		return nil, fmt.Errorf("store: scan conversation row: %w", err)
	}
	c.Status = ConversationStatus(status)
	return c, nil
}
