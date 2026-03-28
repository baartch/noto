package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"noto/internal/backup"
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

// NotesSavingCallback is called when extraction starts.
type NotesSavingCallback func()

// Session manages a single chat session.
type Session struct {
	profileID      string
	profileSlug    string
	conversationID string
	systemPrompt   string
	cacheHit       bool

	convRepo    *store.ConversationRepo
	msgRepo     *store.MessageRepo
	noteRepo    *store.MemoryNoteRepo
	cacheRepo   *store.ContextCacheRepo
	summaryRepo *store.SessionSummaryRepo
	adapter          provider.Adapter
	extractor        *memory.Extractor
	extractorAdapter provider.Adapter
	logger      observe.Logger

	backupStop  chan struct{}
	pendingNotes int
	pendingMu    sync.Mutex
	pendingDone  chan struct{}

	// history is the in-memory context window sent to the provider.
	// It starts with recent messages from the previous session, then
	// grows with messages from the current session.
	history []*store.Message

	onNotes       NotesCallback
	onNotesSaving NotesSavingCallback
	stats         provider.Stats
}

// NewSession creates a new conversation, assembles the system prompt with
// memory notes, and pre-populates history from the previous session.
func NewSession(
	ctx context.Context,
	profileID string,
	profileSlug string,
	baseSystemPrompt string,
	db *store.DB,
	convRepo *store.ConversationRepo,
	msgRepo *store.MessageRepo,
	noteRepo *store.MemoryNoteRepo,
	summaryRepo *store.SessionSummaryRepo,
	adapter provider.Adapter,
	extractorAdapter provider.Adapter,
	logger observe.Logger,
	onNotes NotesCallback,
	onNotesSaving NotesSavingCallback,
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

	s := &Session{
		profileID:      profileID,
		profileSlug:    profileSlug,
		conversationID: convID,
		systemPrompt:   rc.AssembledPrompt,
		cacheHit:       rc.CacheHit,
		convRepo:       convRepo,
		msgRepo:        msgRepo,
		noteRepo:       noteRepo,
		summaryRepo:    summaryRepo,
		adapter:        adapter,
		extractorAdapter: extractorAdapter,
		cacheRepo:       store.NewContextCacheRepo(db),
		extractor:        memory.NewExtractor(noteRepo, adapter, store.NewContextCacheRepo(db)),
		logger:         logger,
		history:        recentHistory,
		onNotes:        onNotes,
		onNotesSaving: onNotesSaving,
		backupStop:     make(chan struct{}),
		pendingDone:    make(chan struct{}),
	}
	if profileSlug != "" {
		s.startBackupTicker()
	}
	return s, nil
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

	// Accumulate token/cost stats.
	s.stats.Add(resp)

	// Fire background extraction — never blocks the reply.
	go s.extractAsync(userMsg, resp.Content)

	return &SendResult{Reply: resp.Content, LatencyMs: latency}, nil
}

// Stats returns a snapshot of current session usage stats.
func (s *Session) Stats() provider.Stats { return s.stats }

// extractAsync runs note extraction and calls onNotes when done.
func (s *Session) extractAsync(userMsg, assistantMsg string) {
	s.markNotesPending()
	if s.onNotesSaving != nil {
		s.onNotesSaving()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	extractor := s.extractor
	if s.extractorAdapter != nil {
		extractor = memory.NewExtractor(s.noteRepo, s.extractorAdapter, s.cacheRepo)
	}
	result, err := extractor.ExtractTurn(ctx, s.profileID, s.conversationID, userMsg, assistantMsg)
	if err != nil {
		s.logger.Errorf("memory extraction failed: %v", err)
		s.markNotesDone(0)
		return
	}
	if s.onNotes != nil {
		s.onNotes(len(result.Notes))
	}
	s.markNotesDone(len(result.Notes))
}

// SetModel updates the model used for subsequent provider calls.
func (s *Session) SetModel(model string) {
	if a, ok := s.adapter.(interface{ SetModel(string) }); ok {
		a.SetModel(model)
	}
}

// SetExtractorModel updates the model used for note extraction.
func (s *Session) SetExtractorModel(model string) {
	if s.extractorAdapter == nil {
		return
	}
	if a, ok := s.extractorAdapter.(interface{ SetModel(string) }); ok {
		a.SetModel(model)
	}
}

// CacheStatus returns a short label for the footer showing whether the
// memory context was served from cache or rebuilt from SQLite at startup.
func (s *Session) CacheStatus() string {
	if s.cacheHit {
		return "ctx:hit"
	}
	return "ctx:miss"
}

// Close archives the conversation, persists a session summary, and snapshots backups.
func (s *Session) Close(ctx context.Context) {
	if s.profileSlug != "" {
		s.stopBackupTicker()
		if err := backup.Snapshot(s.profileSlug); err != nil {
			s.logger.Errorf("backup snapshot failed: %v", err)
		}
	}

	s.waitForPendingNotes(4 * time.Second)
	summaryText := buildSessionSummary(filterConversationMessages(s.history, s.conversationID))
	if summaryText != "" {
		handoff := NewSessionHandoff(s.convRepo, s.summaryRepo)
		err := handoff.Execute(ctx, HandoffInput{
			ProfileID:      s.profileID,
			ConversationID: s.conversationID,
			SummaryText:    summaryText,
			OpenLoops:      "[]",
			NextActions:    "[]",
		})
		if err != nil {
			s.logger.Errorf("session handoff failed: %v", err)
			_ = s.convRepo.Archive(ctx, s.conversationID)
		}
		return
	}
	_ = s.convRepo.Archive(ctx, s.conversationID)
}

func buildSessionSummary(messages []*store.Message) string {
	if len(messages) == 0 {
		return ""
	}
	maxMessages := 6
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}
	var sb strings.Builder
	for _, m := range messages {
		role := "User"
		if m.Role == store.RoleAssistant {
			role = "Assistant"
		}
		sb.WriteString("- " + role + ": " + truncateSummary(m.Content) + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func truncateSummary(text string) string {
	max := 280
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

func filterConversationMessages(messages []*store.Message, conversationID string) []*store.Message {
	var out []*store.Message
	for _, m := range messages {
		if m.ConversationID == conversationID {
			out = append(out, m)
		}
	}
	return out
}

func (s *Session) markNotesPending() {
	s.pendingMu.Lock()
	s.pendingNotes++
	s.pendingMu.Unlock()
}

func (s *Session) markNotesDone(count int) {
	s.pendingMu.Lock()
	if s.pendingNotes > 0 {
		s.pendingNotes--
	}
	remaining := s.pendingNotes
	s.pendingMu.Unlock()

	if remaining == 0 {
		select {
		case s.pendingDone <- struct{}{}:
		default:
		}
	}
}

func (s *Session) waitForPendingNotes(timeout time.Duration) {
	s.pendingMu.Lock()
	pending := s.pendingNotes
	s.pendingMu.Unlock()
	if pending == 0 {
		return
	}

	select {
	case <-s.pendingDone:
	case <-time.After(timeout):
	}
}

func (s *Session) startBackupTicker() {
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := backup.Snapshot(s.profileSlug); err != nil {
					s.logger.Errorf("backup snapshot failed: %v", err)
				}
			case <-s.backupStop:
				return
			}
		}
	}()
}

func (s *Session) stopBackupTicker() {
	select {
	case <-s.backupStop:
		return
	default:
		close(s.backupStop)
	}
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
