package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrMessageNotFound is returned when a message lookup fails.
var ErrMessageNotFound = errors.New("store: message not found")

// MessageRole defines the turn role in a conversation.
type MessageRole string

// Known message roles.
const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// Message is the data model for a single conversation turn.
type Message struct {
	ID             string
	ConversationID string
	Role           MessageRole
	Content        string
	Provider       string
	Model          string
	CreatedAt      time.Time
}

// MessageRepo manages CRUD operations for messages.
type MessageRepo struct {
	db *DB
}

// NewMessageRepo creates a new MessageRepo.
func NewMessageRepo(db *DB) *MessageRepo {
	return &MessageRepo{db: db}
}

// Create inserts a new message.
func (r *MessageRepo) Create(ctx context.Context, m *Message) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO messages (id, conversation_id, role, content, provider, model)
		VALUES (?, ?, ?, ?, ?, ?)
	`, m.ID, m.ConversationID, string(m.Role), m.Content, m.Provider, m.Model)
	if err != nil {
		return fmt.Errorf("store: create message: %w", err)
	}
	return nil
}

// ListByConversation returns all messages for a conversation in chronological order.
func (r *MessageRepo) ListByConversation(ctx context.Context, conversationID string) ([]*Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, conversation_id, role, content, provider, model, created_at
		FROM messages WHERE conversation_id = ?
		ORDER BY created_at ASC
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: list messages: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		var role string
		err := rows.Scan(
			&m.ID, &m.ConversationID, &role, &m.Content,
			&m.Provider, &m.Model, &m.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("store: scan message: %w", err)
		}
		m.Role = MessageRole(role)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// CountByConversation returns the number of messages in a conversation.
func (r *MessageRepo) CountByConversation(ctx context.Context, conversationID string) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, conversationID,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count messages: %w", err)
	}
	return n, nil
}

// ListRecentUserMessages returns recent user message contents for a profile.
func (r *MessageRepo) ListRecentUserMessages(ctx context.Context, profileID string, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.content
		FROM messages m
		JOIN conversations c ON m.conversation_id = c.id
		WHERE c.profile_id = ? AND m.role = 'user'
		ORDER BY m.created_at DESC
		LIMIT ?
	`, profileID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list recent user messages: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var out []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("store: scan recent user message: %w", err)
		}
		out = append(out, content)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
