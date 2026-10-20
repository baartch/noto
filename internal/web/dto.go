package web

import "time"

// StateDTO contains browser-safe server and active-profile state.
type StateDTO struct {
	Profile       ProfileDTO `json:"profile"`
	Model         string     `json:"model"`
	SetupRequired bool       `json:"setup_required"`
	Sending       bool       `json:"sending"`
	PendingNotes  int        `json:"pending_notes"`
}

// ProfileDTO exposes only the profile label needed by the workspace.
type ProfileDTO struct {
	Name string `json:"name"`
}

// MessageDTO is a sanitized, profile-scoped chat message for the browser.
type MessageDTO struct {
	ID                    string    `json:"id"`
	ConversationID        string    `json:"conversation_id"`
	ConversationStartedAt time.Time `json:"conversation_started_at"`
	Role                  string    `json:"role"`
	Content               string    `json:"content"`
	ContentHTML           string    `json:"content_html"`
	CreatedAt             time.Time `json:"created_at"`
	Model                 string    `json:"model,omitempty"`
}

// MessagePageDTO contains a chronological page and optional older-page cursor.
type MessagePageDTO struct {
	Items      []MessageDTO `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

// MemoryDTO is either an individual note or a weekly/monthly summary artifact.
type MemoryDTO struct {
	ID             string    `json:"id"`
	Content        string    `json:"content"`
	ContentHTML    string    `json:"content_html"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
	Category       string    `json:"category,omitempty"`
	SummaryType    string    `json:"summary_type,omitempty"`
	PeriodKey      string    `json:"period_key,omitempty"`
	PeriodStart    time.Time `json:"period_start,omitzero"`
	PeriodEnd      time.Time `json:"period_end,omitzero"`
	FreshnessState string    `json:"freshness_state,omitempty"`
}

// MemoryPageDTO contains one tab's memory items and optional next offset.
type MemoryPageDTO struct {
	Items      []MemoryDTO `json:"items"`
	NextOffset *int        `json:"next_offset"`
}

// ReplyDTO contains the plain and sanitized-rendered provider reply.
type ReplyDTO struct {
	Content     string `json:"content"`
	ContentHTML string `json:"content_html"`
}

// SendResponseDTO identifies the request and its completed reply.
type SendResponseDTO struct {
	RequestID string   `json:"request_id"`
	Reply     ReplyDTO `json:"reply"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
