package chat

import (
	"context"
	"fmt"
	"time"

	"noto/internal/memory"
	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
)

// NotesCallback is called after extraction completes. count is the number of
// new notes saved (may be 0). Used by the TUI to show a subtle indicator.
type NotesCallback func(count int)

// Session manages a single chat session: conversation lifecycle, message
// persistence, context assembly, provider calls, and background note extraction.
type Session struct {
	profileID      string
	conversationID string
	systemPrompt   string // assembled at start: base prompt + memory notes

	convRepo    *store.ConversationRepo
	msgRepo     *store.MessageRepo
	noteRepo    *store.MemoryNoteRepo
	summaryRepo *store.SessionSummaryRepo
	adapter     provider.Adapter
	extractor   *memory.Extractor
	retrieval   *memory.Retrieval
	logger      observe.Logger

	// history holds the in-memory message list for the context window.
	history []*store.Message

	onNotes NotesCallback
}

// NewSession creates and persists a new conversation, assembles the system
// prompt from stored notes, and returns a ready-to-use Session.
func NewSession(
	ctx context.Context,
	profileID string,
	baseSystemPrompt string,
	convRepo *store.ConversationRepo,
	msgRepo *store.MessageRepo,
	noteRepo *store.MemoryNoteRepo,
	summaryRepo *store.SessionSummaryRepo,
	adapter provider.Adapter,
	logger observe.Logger,
	onNotes NotesCallback,
) (*Session, error) {
	// Assemble system prompt with memory context.
	ret := memory.NewRetrieval(noteRepo, summaryRepo)
	rc, err := ret.Assemble(ctx, profileID, baseSystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("session: assemble context: %w", err)
	}

	// Create conversation record.
	convID := fmt.Sprintf("conv-%x", time.Now().UnixNano())
	conv := &store.Conversation{
		ID:        convID,
		ProfileID: profileID,
		Status:    store.ConversationActive,
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("session: create conversation: %w", err)
	}

	extractor := memory.NewExtractor(noteRepo, adapter)

	return &Session{
		profileID:      profileID,
		conversationID: convID,
		systemPrompt:   rc.AssembledPrompt,
		convRepo:       convRepo,
		msgRepo:        msgRepo,
		noteRepo:       noteRepo,
		summaryRepo:    summaryRepo,
		adapter:        adapter,
		extractor:      extractor,
		retrieval:      ret,
		logger:         logger,
		onNotes:        onNotes,
	}, nil
}

// SendResult is the outcome of a single chat turn.
type SendResult struct {
	Reply     string
	LatencyMs int64
}

// Send sends a user message, persists both turns, calls the provider, and
// triggers background note extraction. It never blocks on extraction.
func (s *Session) Send(ctx context.Context, userMsg string) (*SendResult, error) {
	start := time.Now()

	// Persist user message.
	userMsgRec := &store.Message{
		ID:             fmt.Sprintf("msg-%x", time.Now().UnixNano()),
		ConversationID: s.conversationID,
		Role:           store.RoleUser,
		Content:        userMsg,
	}
	if err := s.msgRepo.Create(ctx, userMsgRec); err != nil {
		return nil, fmt.Errorf("session: persist user message: %w", err)
	}
	s.history = append(s.history, userMsgRec)

	// Build provider request: system prompt + full history.
	msgs := make([]provider.Message, 0, len(s.history)+1)
	msgs = append(msgs, provider.Message{Role: "system", Content: s.systemPrompt})
	for _, m := range s.history {
		msgs = append(msgs, provider.Message{Role: string(m.Role), Content: m.Content})
	}

	resp, err := s.adapter.Complete(ctx, provider.CompletionRequest{
		Messages:    msgs,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("session: provider call: %w", err)
	}

	latency := time.Since(start).Milliseconds()

	// Persist assistant message.
	asstMsgRec := &store.Message{
		ID:             fmt.Sprintf("msg-%x", time.Now().UnixNano()),
		ConversationID: s.conversationID,
		Role:           store.RoleAssistant,
		Content:        resp.Content,
		Provider:       s.adapter.ProviderType(),
		Model:          resp.Model,
	}
	if err := s.msgRepo.Create(ctx, asstMsgRec); err != nil {
		return nil, fmt.Errorf("session: persist assistant message: %w", err)
	}
	s.history = append(s.history, asstMsgRec)

	// Fire background extraction — never blocks the reply.
	go s.extractAsync(userMsg, resp.Content)

	return &SendResult{Reply: resp.Content, LatencyMs: latency}, nil
}

// extractAsync runs note extraction in a goroutine and calls onNotes when done.
func (s *Session) extractAsync(userMsg, assistantMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := s.extractor.ExtractTurn(ctx, s.profileID, s.conversationID, userMsg, assistantMsg)
	if err != nil {
		s.logger.Errorf("memory extraction: %v", err)
		return
	}
	if s.onNotes != nil {
		s.onNotes(len(result.Notes))
	}
}

// SetModel updates the model used for subsequent provider calls.
// This allows /model changes to take effect mid-session.
func (s *Session) SetModel(model string) {
	if a, ok := s.adapter.(interface{ SetModel(string) }); ok {
		a.SetModel(model)
	}
}

// Close archives the conversation. Call when the user exits chat.
func (s *Session) Close(ctx context.Context) {
	_ = s.convRepo.Archive(ctx, s.conversationID)
}

// NoteCount returns how many memory notes exist for this profile.
func (s *Session) NoteCount(ctx context.Context) int {
	notes, err := s.noteRepo.ListByProfile(ctx, s.profileID)
	if err != nil {
		return 0
	}
	return len(notes)
}
