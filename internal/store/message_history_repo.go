package store

import (
	"context"
	"fmt"
	"time"
)

// MessageHistoryCursor identifies a stable position in profile-wide history.
type MessageHistoryCursor struct {
	CreatedAt time.Time
	RowID     int64
}

// MessageHistoryPage contains chronological messages and a cursor for older rows.
type MessageHistoryPage struct {
	Messages []*Message
	Next     *MessageHistoryCursor
}

// ListHistoryByProfile returns one chronological page across all conversations
// in a profile, seeking strictly before the composite timestamp/rowid cursor.
func (r *MessageRepo) ListHistoryByProfile(ctx context.Context, profileID string, before *MessageHistoryCursor, limit int) (MessageHistoryPage, error) {
	if limit <= 0 {
		return MessageHistoryPage{Messages: []*Message{}}, nil
	}

	query := `
		SELECT m.rowid, m.id, m.conversation_id, c.started_at, m.role, m.content, m.provider, m.model, m.created_at, m.reference_influenced
		FROM messages m
		JOIN conversations c ON m.conversation_id = c.id
		WHERE c.profile_id = ?
	`
	args := []any{profileID}
	if before != nil {
		query += ` AND (m.created_at < ? OR (m.created_at = ? AND m.rowid < ?))`
		createdAt := before.CreatedAt.UTC()
		args = append(args, createdAt, createdAt, before.RowID)
	}
	query += ` ORDER BY m.created_at DESC, m.rowid DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return MessageHistoryPage{}, fmt.Errorf("store: list profile history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type historyRow struct {
		message *Message
		rowID   int64
	}
	descending := make([]historyRow, 0, limit+1)
	for rows.Next() {
		m := &Message{}
		var rowID int64
		var role string
		if err := rows.Scan(&rowID, &m.ID, &m.ConversationID, &m.ConversationStartedAt, &role, &m.Content, &m.Provider, &m.Model, &m.CreatedAt, &m.ReferenceInfluenced); err != nil {
			return MessageHistoryPage{}, fmt.Errorf("store: scan profile history: %w", err)
		}
		m.Role = MessageRole(role)
		descending = append(descending, historyRow{message: m, rowID: rowID})
	}
	if err := rows.Err(); err != nil {
		return MessageHistoryPage{}, fmt.Errorf("store: iterate profile history: %w", err)
	}

	hasMore := len(descending) > limit
	if hasMore {
		descending = descending[:limit]
	}
	page := MessageHistoryPage{Messages: make([]*Message, len(descending))}
	for i := range descending {
		page.Messages[len(descending)-1-i] = descending[i].message
	}
	if hasMore {
		oldest := descending[len(descending)-1]
		page.Next = &MessageHistoryCursor{CreatedAt: oldest.message.CreatedAt, RowID: oldest.rowID}
	}
	return page, nil
}
