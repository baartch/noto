package chat

import (
	"context"
	"fmt"
	"time"

	"noto/internal/store"
)

// SessionHandoff captures a session summary and archives the conversation at session end.
type SessionHandoff struct {
	convRepo    *store.ConversationRepo
	summaryRepo *store.SessionSummaryRepo
}

// NewSessionHandoff creates a SessionHandoff.
func NewSessionHandoff(
	convRepo *store.ConversationRepo,
	summaryRepo *store.SessionSummaryRepo,
) *SessionHandoff {
	return &SessionHandoff{
		convRepo:    convRepo,
		summaryRepo: summaryRepo,
	}
}

// HandoffInput contains the data needed to produce a session summary.
type HandoffInput struct {
	ProfileID      string
	ConversationID string
	SummaryText    string
	OpenLoops      string // JSON array
	NextActions    string // JSON array
}

// Execute archives the conversation and persists a session summary.
func (h *SessionHandoff) Execute(ctx context.Context, input HandoffInput) error {
	summary := &store.SessionSummary{
		ID:             fmt.Sprintf("ss-%x", time.Now().UnixNano()),
		ProfileID:      input.ProfileID,
		ConversationID: input.ConversationID,
		SummaryText:    input.SummaryText,
		OpenLoops:      input.OpenLoops,
		NextActions:    input.NextActions,
	}
	if err := h.summaryRepo.Create(ctx, summary); err != nil {
		return fmt.Errorf("chat: persist session summary: %w", err)
	}
	if err := h.convRepo.Archive(ctx, input.ConversationID); err != nil {
		return fmt.Errorf("chat: archive conversation: %w", err)
	}
	return nil
}
