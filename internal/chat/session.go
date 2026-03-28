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

const (
	// recentHistoryMessages is the number of messages from the most recent
	// previous conversation to prepend to context on session start.
	recentHistoryMessages = 20
)

// NotesCallback is called after extraction completes.
type NotesCallback func(count int)

// Session manages a single chat session.
type Session struct {
	profileID      string
	conversationID string
	systemPrompt   string

	convRepo    *store.ConversationRepo
	msgRepo     *store.MessageRepo
	noteRepo    *store.MemoryNoteRepo
	summaryRepo *store.SessionSummaryRepo
	adapter     provider.Adapter
	extractor   *memory.Extractor
	logger      observe.Logger

	// history is the in-memory context window sent to the provider.
	// It starts with recent messages from the previous session, then
	// grows with messages from the current session.
	history []*store.Message

	onNotes NotesCallback
}

// NewSession creates a new conversation, assembles the system prompt with
// memory notes, and pre-populates history from the previous session.
func NewSession(
	ctx context.Context,
	profileID string,
	baseSystemPrompt string,
	db *store.DB,
	convRepo *store.ConversationRepo,
	msgRepo *store.MessageRepo,
	noteRepo *store.MemoryNoteRepo,
	summaryRepo *store.SessionSummaryRepo,
	adapter provider.Adapter,
	logger observe.Logger,
	onNotes NotesCallback,
) (*Session, error) {
	// Build system prompt with injected memory notes + session summary.
	cacheRepo := store.NewContextCacheRepo(db)
	ret := memory.NewRetrieval(noteRepo, summaryRepo, cacheRepo)
	rc, err := ret.Assemble(ctx, profileID, baseSystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("session: assemble context: %w", err)
	}

	// Load recent messages from the previous conversation for context continuity.
	recentHistory, err := loadRecentHistory(ctx, profileID, convRepo, msgRepo)
	if err != nil {
		// Non-fatal — proceed without history.
		logger.Errorf("session: load recent history: %v", err)
		recentHistory = nil
	}

	// Create the new conversation record.
	convID := fmt.Sprintf("conv-%x", time.Now().UnixNano())
	if err := convRepo.Create(ctx, &store.Conversation{
		ID:        convID,
		ProfileID: profileID,
		Status:    store.ConversationActive,
	}); err != nil {
		return nil, fmt.Errorf("session: create conversation: %w", err)
	}

	return &Session{
		profileID:      profileID,
		conversationID: convID,
		systemPrompt:   rc.AssembledPrompt,
		convRepo:       convRepo,
		msgRepo:        msgRepo,
		noteRepo:       noteRepo,
		summaryRepo:    summaryRepo,
		adapter:        adapter,
		extractor:      memory.NewExtractor(noteRepo, adapter, store.NewContextCacheRepo(db)),
		logger:         logger,
		history:        recentHistory,
		onNotes:        onNotes,
	}, nil
}

// SendResult is the outcome of a single chat turn.
type SendResult struct {
	Reply     string
	LatencyMs int64
}

// Send sends a user message, persists both turns, calls the provider, and
// triggers background note extraction.
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

	// Build provider request.
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

// extractAsync runs note extraction and calls onNotes when done.
func (s *Session) extractAsync(userMsg, assistantMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result, err := s.extractor.ExtractTurn(ctx, s.profileID, s.conversationID, userMsg, assistantMsg)
	if err != nil {
		s.logger.Errorf("memory extraction failed: %v", err)
		return
	}
	if s.onNotes != nil {
		s.onNotes(len(result.Notes))
	}
}

// SetModel updates the model used for subsequent provider calls.
func (s *Session) SetModel(model string) {
	if a, ok := s.adapter.(interface{ SetModel(string) }); ok {
		a.SetModel(model)
	}
}

// Close archives the conversation.
func (s *Session) Close(ctx context.Context) {
	_ = s.convRepo.Archive(ctx, s.conversationID)
}

// loadRecentHistory returns the last N messages from the most recent archived
// conversation for this profile, to seed the context window.
func loadRecentHistory(
	ctx context.Context,
	profileID string,
	convRepo *store.ConversationRepo,
	msgRepo *store.MessageRepo,
) ([]*store.Message, error) {
	convs, err := convRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}

	// Find the most recent archived conversation.
	var prevConvID string
	for _, c := range convs {
		if c.Status == store.ConversationArchived {
			prevConvID = c.ID
			break // ListByProfile returns newest first
		}
	}
	if prevConvID == "" {
		return nil, nil // no previous session
	}

	msgs, err := msgRepo.ListByConversation(ctx, prevConvID)
	if err != nil {
		return nil, err
	}

	// Take only the last N messages to avoid bloating the context window.
	if len(msgs) > recentHistoryMessages {
		msgs = msgs[len(msgs)-recentHistoryMessages:]
	}
	return msgs, nil
}
