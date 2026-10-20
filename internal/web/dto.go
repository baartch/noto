package web

import "time"

type StateDTO struct {
	Profile       ProfileDTO `json:"profile"`
	Model         string     `json:"model"`
	SetupRequired bool       `json:"setup_required"`
	Sending       bool       `json:"sending"`
	PendingNotes  int        `json:"pending_notes"`
}

type ProfileDTO struct {
	Name string `json:"name"`
}

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

type MessagePageDTO struct {
	Items      []MessageDTO `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

type MemoryDTO struct {
	ID             string    `json:"id"`
	Content        string    `json:"content"`
	ContentHTML    string    `json:"content_html"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	Category       string    `json:"category,omitempty"`
	SummaryType    string    `json:"summary_type,omitempty"`
	PeriodKey      string    `json:"period_key,omitempty"`
	PeriodStart    time.Time `json:"period_start,omitempty"`
	PeriodEnd      time.Time `json:"period_end,omitempty"`
	FreshnessState string    `json:"freshness_state,omitempty"`
}

type MemoryPageDTO struct {
	Items      []MemoryDTO `json:"items"`
	NextOffset *int        `json:"next_offset"`
}

type ReplyDTO struct {
	Content     string `json:"content"`
	ContentHTML string `json:"content_html"`
}

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
