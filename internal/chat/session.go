package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"noto/internal/backup"
	"noto/internal/config"
	"noto/internal/memory"
	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/vector"
	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

const (
	// recentHistoryMessages is the number of messages from the most recent
	// previous conversation to prepend to context on session start.
	recentHistoryMessages = 20
)

// NotesCallback is called after extraction completes.
type NotesCallback func(saved, updated int)

// NotesSavingCallback is called when extraction starts.
type NotesSavingCallback func()

// ContextStatusCallback is called when context status changes.
type ContextStatusCallback func(status string)

// Session manages a single chat session.
type Session struct {
	profileID      string
	profileSlug    string
	conversationID string
	systemPrompt   string
	cacheHit       bool
	cacheTier      string
	cacheStale     bool
	cacheReval     bool
	cacheMiss      string

	convRepo          *store.ConversationRepo
	msgRepo           *store.MessageRepo
	noteRepo          *store.MemoryNoteRepo
	cacheRepo         *store.ContextCacheRepo
	adapter           provider.Adapter
	retrieval         *memory.Retrieval
	extractor         *memory.Extractor
	extractorAdapter  provider.Adapter
	logger            observe.Logger
	baseSystemPrompt  string
	db                *store.DB
	vectorIndexPath   string
	vectorIndex       vector.Index
	extractorFallback bool
	embeddingModel    string
	missingEmbedding  bool
	toolsEnabled      bool
	modelContextMax   int

	backupStop   chan struct{}
	pendingNotes int
	pendingMu    sync.Mutex
	pendingDone  chan struct{}

	// history is the in-memory context window sent to the provider.
	// It starts with recent messages from the previous session, then
	// grows with messages from the current session.
	history []*store.Message

	onNotes         NotesCallback
	onNotesSaving   NotesSavingCallback
	onContextStatus ContextStatusCallback
	onStats         func(provider.Stats)
	stats           provider.Stats
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
	adapter provider.Adapter,
	extractorAdapter provider.Adapter,
	logger observe.Logger,
	onNotes NotesCallback,
	onNotesSaving NotesSavingCallback,
	onContextStatus ContextStatusCallback,
	onStats func(provider.Stats),
) (*Session, error) {
	// Build system prompt with injected memory context.
	cacheRepo := store.NewContextCacheRepo(db)
	vecPath, _ := config.ProfileVectorPath(profileSlug)
	embeddingModel := ""
	if db != nil {
		cfgRepo := store.NewProviderConfigRepo(db)
		if cfg, err := cfgRepo.GetActive(ctx, profileID); err == nil {
			embeddingModel = cfg.EmbeddingsModel
		}
	}
	var retrievalEmbedder vector.Embedder
	if embeddingModel != "" {
		retrievalEmbedder = adapter
	}
	ret := memory.NewRetrieval(
		noteRepo,
		cacheRepo,
		memory.WithVectorIndexPath(vecPath),
		memory.WithWarnFunc(func(err error) {
			logger.Infof("vector index issue: %v", err)
		}),
		memory.WithTimeline(store.NewTimelineSettingsRepo(db), store.NewMemorySummaryRepo(db)),
		memory.WithVectorRetrieval(vector.NewFileIndex(vecPath, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0)), profileID, retrievalEmbedder, embeddingModel),
	)
	rc, err := ret.Assemble(ctx, profileID, baseSystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("session: assemble context: %w", err)
	}
	logger.Infof("context retrieval: tier=%s hit=%t stale=%t revalidate=%t miss_reason=%s", rc.CacheTier, rc.CacheHit, rc.ServedStale, rc.RevalidationStarted, rc.MissReason)

	// Load recent messages from the previous conversation for context continuity.
	recentHistory, err := loadRecentHistory(ctx, profileID, convRepo, msgRepo)
	if err != nil {
		// Non-fatal — proceed without history.
		logger.Errorf("session: load recent history: %v", err)
		recentHistory = nil
	}

	// Repair guard: archive any lingering active conversations before creating a new one.
	if archived, err := convRepo.ArchiveActiveByProfile(ctx, profileID); err != nil {
		logger.Errorf("session: archive active conversations: %v", err)
	} else if archived > 0 {
		logger.Infof("session: archived %d lingering active conversation(s)", archived)
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
		profileID:         profileID,
		profileSlug:       profileSlug,
		conversationID:    convID,
		systemPrompt:      rc.AssembledPrompt,
		cacheHit:          rc.CacheHit,
		cacheTier:         rc.CacheTier,
		cacheStale:        rc.ServedStale,
		cacheReval:        rc.RevalidationStarted,
		cacheMiss:         rc.MissReason,
		retrieval:         ret,
		convRepo:          convRepo,
		msgRepo:           msgRepo,
		noteRepo:          noteRepo,
		adapter:           adapter,
		extractorAdapter:  extractorAdapter,
		cacheRepo:         store.NewContextCacheRepo(db),
		extractor:         memory.NewExtractor(noteRepo, adapter, store.NewContextCacheRepo(db)),
		logger:            logger,
		history:           recentHistory,
		onNotes:           onNotes,
		onNotesSaving:     onNotesSaving,
		onContextStatus:   onContextStatus,
		onStats:           onStats,
		backupStop:        make(chan struct{}),
		pendingDone:       make(chan struct{}),
		baseSystemPrompt:  baseSystemPrompt,
		db:                db,
		vectorIndexPath:   vecPath,
		vectorIndex:       vector.NewFileIndex(vecPath, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0)),
		extractorFallback: extractorAdapter == nil && adapter != nil,
		embeddingModel:    embeddingModel,
		missingEmbedding:  embeddingModel == "",
	}
	if s.onContextStatus != nil {
		s.onContextStatus(s.CacheStatus())
	}
	if profileSlug != "" {
		s.startBackupTicker()
	}

	// Rebuild vector index from existing notes if the index file is missing.
	// This avoids "index file not found" warnings on every turn and ensures
	// the vector deduper works from the first extraction.
	if s.noteRepo != nil && s.embeddingModel != "" && s.adapter != nil && s.vectorIndex != nil {
		if _, err := os.Stat(vecPath); os.IsNotExist(err) {
			existingNotes, listErr := s.noteRepo.ListByProfile(ctx, profileID)
			if listErr == nil && len(existingNotes) > 0 {
				if syncErr := s.syncVectorIndex(ctx, existingNotes); syncErr != nil {
					logger.Infof("initial vector index rebuild: %v", syncErr)
				}
			}
		}
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

	// Inject current date/time into user message for LLM awareness
	injectedMsg := injectDateTime(userMsg)

	// Build provider request from retrieval cache/context each turn.
	if s.db != nil {
		builder := memory.NewSummaryRollupBuilder(s.noteRepo, store.NewMemorySummaryRepo(s.db))
		if _, err := builder.CatchUp(ctx, s.profileID, time.Now().UTC()); err != nil {
			s.logger.Errorf("summary catch-up failed: %v", err)
		}
		if err := builder.RegenerateStale(ctx, s.profileID, time.Now().UTC()); err != nil {
			s.logger.Errorf("summary regeneration failed: %v", err)
		}
	}
	systemPrompt := s.systemPrompt
	if s.retrieval != nil {
		rc, err := s.retrieval.Assemble(ctx, s.profileID, s.baseSystemPrompt)
		if err != nil {
			s.logger.Errorf("context retrieval (turn) failed: %v", err)
		} else {
			systemPrompt = rc.AssembledPrompt
			s.systemPrompt = rc.AssembledPrompt
			s.cacheHit = rc.CacheHit
			s.cacheTier = rc.CacheTier
			s.cacheStale = rc.ServedStale
			s.cacheReval = rc.RevalidationStarted
			s.cacheMiss = rc.MissReason
			s.logger.Infof("context retrieval (turn): tier=%s hit=%t stale=%t revalidate=%t miss_reason=%s", rc.CacheTier, rc.CacheHit, rc.ServedStale, rc.RevalidationStarted, rc.MissReason)
		}
	}
	if s.onContextStatus != nil {
		s.onContextStatus(s.CacheStatus())
	}
	msgs := make([]provider.Message, 0, len(s.history)+1)
	msgs = append(msgs, provider.Message{Role: "system", Content: systemPrompt})
	for i, m := range s.history {
		content := m.Content
		// Use injected message for the most recent user message
		if m.Role == store.RoleUser && i == len(s.history)-1 {
			content = injectedMsg
		}
		msgs = append(msgs, provider.Message{Role: string(m.Role), Content: content})
	}

	req := provider.CompletionRequest{
		Messages:    msgs,
		Temperature: 0.7,
	}
	if s.toolsEnabled {
		req.Tools = []provider.ToolDefinition{
			{Type: "function", Name: "search_memory_keywords", Description: "Search memory notes by keyword query. Results are objects with content, category, created_at, and importance.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Keyword or short phrase to search for in memory notes"}}, "required": []string{"query"}}},
			{Type: "function", Name: "search_memory_time_range", Description: "Search memory notes within a time range. Results are objects with content, category, created_at, and importance.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"start_time": map[string]any{"type": "string", "description": "Start of the time range in RFC3339 format"}, "end_time": map[string]any{"type": "string", "description": "End of the time range in RFC3339 format"}}, "required": []string{"start_time", "end_time"}}},
		}
		s.logger.Infof("tools: enabled for chat request with %d tool definitions", len(req.Tools))
	} else {
		s.logger.Infof("tools: disabled for chat request")
	}

	resp, err := s.adapter.Complete(ctx, req)
	if err == nil {
		s.logger.Infof("tools: provider returned %d tool call(s)", len(resp.ToolCalls))
	}
	if err == nil && len(resp.ToolCalls) > 0 && s.toolsEnabled {
		followup := append([]provider.Message{}, req.Messages...)
		pipeline := NewPipeline(s.convRepo, s.msgRepo, s.adapter, s.logger).WithMemorySearchTools(s.noteRepo, store.NewMemorySummaryRepo(s.db)).WithToolsEnabled(true)
		for _, call := range resp.ToolCalls {
			s.logger.Infof("tools: executing tool call name=%s call_id=%s args=%s", call.Name, call.CallID, call.Arguments)
			followup = append(followup, provider.Message{Role: "assistant", Content: call.Name, ToolCallID: call.CallID, ToolCallName: call.Name, ToolCallArguments: call.Arguments, ToolID: call.ID})
			result := pipeline.executeToolCall(ctx, s.profileID, call)
			s.logger.Infof("tools: tool call completed name=%s call_id=%s", call.Name, call.CallID)
			followup = append(followup, provider.Message{Role: "tool", Content: result, ToolCallID: call.CallID})
		}
		resp, err = s.adapter.Complete(ctx, provider.CompletionRequest{Model: req.Model, Messages: followup, Temperature: req.Temperature, Tools: req.Tools})
		if err != nil {
			s.logger.Errorf("provider follow-up call failed: %v", err)
		} else {
			s.logger.Infof("tools: follow-up provider call completed after tool execution")
		}
	}
	if err != nil {
		s.logger.Errorf("provider call failed: %v", err)
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

	if resp.ContextMax == 0 && s.modelContextMax > 0 {
		resp.ContextMax = s.modelContextMax
	}

	// Accumulate token/cost stats.
	s.stats.AddCompletion(resp)
	if s.onStats != nil {
		s.onStats(s.stats)
	}

	// Fire background extraction — never blocks the reply.
	go s.extractAsync(userMsg, resp.Content)

	return &SendResult{Reply: resp.Content, LatencyMs: latency}, nil
}

// Stats returns a snapshot of current session usage stats.
func (s *Session) Stats() provider.Stats { return s.stats }

// extractAsync runs note extraction and calls onNotes when done.
func (s *Session) syncVectorIndex(ctx context.Context, notes []*store.MemoryNote) error {
	if s.vectorIndex == nil || s.adapter == nil {
		return nil
	}
	if s.embeddingModel == "" {
		s.missingEmbedding = true
		return nil
	}
	fileIndex, ok := s.vectorIndex.(*vector.FileIndex)
	if ok {
		fileIndex.WithProfile(s.profileID)
		if err := fileIndex.Load(); err != nil {
			if errors.Is(err, vector.ErrIndexNotFound) || errors.Is(err, vector.ErrIndexCorrupted) {
				// No existing index; proceed to create a fresh one.
			} else {
				return err
			}
		}
	}

	manifestRepo := store.NewVectorManifestRepo(s.db)
	if err := s.ensureVectorManifest(ctx, manifestRepo); err != nil {
		return err
	}
	var embedder vector.Embedder
	if s.adapter != nil {
		embedder = &usageReportingEmbedder{embedder: s.adapter, onUsage: s.recordAuxUsage}
	}
	syncer := vector.NewSyncer(s.vectorIndex, s.profileID, embedder, s.embeddingModel).WithManifest(manifestRepo)
	records := make([]vector.MemoryNoteRecord, 0, len(notes))
	for _, note := range notes {
		records = append(records, vector.MemoryNoteRecord{ID: note.ID, Content: note.Content})
	}
	if err := syncer.SyncNotes(ctx, records); err != nil {
		return err
	}
	return nil
}

func (s *Session) ensureVectorManifest(ctx context.Context, repo *store.VectorManifestRepo) error {
	_, err := repo.GetManifest(ctx, s.profileID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrManifestNotFound) {
		return err
	}
	return repo.UpsertManifest(ctx, &store.VectorManifest{
		ID:                 fmt.Sprintf("vm-%x", time.Now().UnixNano()),
		ProfileID:          s.profileID,
		IndexPath:          s.vectorIndexPath,
		IndexFormatVersion: "1",
		EmbeddingModel:     s.embeddingModel,
		EmbeddingDim:       0,
		SourceStateVersion: "",
		Status:             store.VectorManifestReady,
	})
}

func (s *Session) ensureVectorCompaction(ctx context.Context) error {
	if s.vectorIndex == nil || s.adapter == nil {
		return nil
	}
	manifestRepo := store.NewVectorManifestRepo(s.db)
	manifest, err := manifestRepo.GetManifest(ctx, s.profileID)
	if err != nil {
		return nil
	}
	if manifest.Status != store.VectorManifestStale && manifest.Status != store.VectorManifestFailed {
		return nil
	}

	notes, err := s.noteRepo.ListByProfile(ctx, s.profileID)
	if err != nil {
		return err
	}
	records := make([]vector.MemoryNoteRecord, 0, len(notes))
	for _, note := range notes {
		records = append(records, vector.MemoryNoteRecord{ID: note.ID, Content: note.Content})
	}

	var embedder vector.Embedder
	if s.adapter != nil {
		embedder = &usageReportingEmbedder{embedder: s.adapter, onUsage: s.recordAuxUsage}
	}
	rebuilder := vector.NewRebuilder(manifestRepo, s.vectorIndex, s.profileID).
		WithManifest(manifestRepo).
		WithEmbedder(embedder, s.embeddingModel)
	return rebuilder.Rebuild(ctx, records)
}

func (s *Session) extractAsync(userMsg, assistantMsg string) {
	s.markNotesPending()
	if s.onNotesSaving != nil {
		s.onNotesSaving()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	extractor := s.extractor
	var embedder provider.Adapter
	if s.extractorAdapter != nil {
		extractor = memory.NewExtractor(s.noteRepo, s.extractorAdapter, s.cacheRepo)
		embedder = s.extractorAdapter
	} else if s.adapter != nil {
		extractor = memory.NewExtractor(s.noteRepo, s.adapter, s.cacheRepo)
		embedder = s.adapter
	}
	if embedder != nil && s.vectorIndex != nil && s.embeddingModel != "" {
		dedupEmbedder := &usageReportingEmbedder{embedder: embedder, onUsage: s.recordAuxUsage}
		deduper := vector.NewVectorDeduper(s.vectorIndex, s.profileID, dedupEmbedder, s.embeddingModel).WithWarnFunc(func(err error) {
			s.logger.Infof("vector dedup issue: %v", err)
		})
		extractor.WithDeduper(deduper)
	}
	extractor.WithLogHook(&sessionLogHook{logger: s.logger})
	extractor.WithUsageHook(s.recordAuxUsage)
	sourceIDs := []string{s.history[len(s.history)-2].ID, s.history[len(s.history)-1].ID}
	result, err := extractor.ExtractTurn(ctx, s.profileID, s.profileSlug, s.conversationID, sourceIDs, userMsg, assistantMsg)
	if err != nil {
		s.logger.Errorf("memory extraction failed: %v", err)
		s.markNotesDone(0)
		return
	}
	s.logger.Infof("extraction result: notes=%d, updated=%d", len(result.Notes), result.Updated)

	if (len(result.Notes) > 0 || len(result.UpdatedNotes) > 0) && s.adapter != nil {
		syncBatch := append([]*store.MemoryNote{}, result.Notes...)
		syncBatch = append(syncBatch, result.UpdatedNotes...)
		if err := s.syncVectorIndex(ctx, syncBatch); err != nil {
			s.logger.Errorf("vector sync failed: %v", err)
		}
	}

	if err := s.ensureVectorCompaction(ctx); err != nil {
		s.logger.Errorf("vector compaction failed: %v", err)
	}

	if s.onNotes != nil {
		s.onNotes(len(result.Notes), result.Updated)
	}
	s.markNotesDone(len(result.Notes) + result.Updated)
}

// SetModel updates the model used for subsequent provider calls.
func (s *Session) SetModel(model string) {
	if a, ok := s.adapter.(interface{ SetModel(string) }); ok {
		a.SetModel(model)
	}
}

// SetToolsEnabled updates whether tool definitions should be exposed for chat.
func (s *Session) SetToolsEnabled(enabled bool) {
	s.toolsEnabled = enabled
}

// SetModelContextMax updates the known context window size for the active model.
func (s *Session) SetModelContextMax(contextMax int) {
	s.modelContextMax = contextMax
	if contextMax > 0 {
		s.stats.ContextMax = contextMax
	}
}

// SetEmbeddingModel updates the embeddings model used for vector operations.
func (s *Session) SetEmbeddingModel(model string) {
	s.embeddingModel = model
	s.missingEmbedding = model == ""
	if s.vectorIndex != nil {
		if fileIndex, ok := s.vectorIndex.(*vector.FileIndex); ok {
			fileIndex.WithProfile(s.profileID)
			_ = fileIndex.Load()
		}
	}
	if s.db != nil {
		manifestRepo := store.NewVectorManifestRepo(s.db)
		if err := manifestRepo.SetManifestEmbeddingModel(context.Background(), s.profileID, model); err != nil {
			_ = manifestRepo.UpsertManifest(context.Background(), &store.VectorManifest{
				ID:                 fmt.Sprintf("vm-%x", time.Now().UnixNano()),
				ProfileID:          s.profileID,
				IndexPath:          s.vectorIndexPath,
				IndexFormatVersion: "1",
				EmbeddingModel:     model,
				EmbeddingDim:       0,
				SourceStateVersion: "",
				Status:             store.VectorManifestReady,
			})
		}
	}
}

// SetExtractorModel updates the model used for note extraction.
func (s *Session) SetExtractorModel(model string) {
	if s.extractorAdapter == nil {
		s.extractorFallback = false
		return
	}
	if a, ok := s.extractorAdapter.(interface{ SetModel(string) }); ok {
		a.SetModel(model)
	}
	s.extractorFallback = false
}

// CacheStatus returns a short label for the footer showing current context state.
func (s *Session) CacheStatus() string {
	if s.cacheStale && s.cacheReval {
		return "ctx:swr"
	}
	if s.cacheHit {
		switch s.cacheTier {
		case "l1":
			return "ctx:l1-hit"
		case "l2":
			return "ctx:l2-hit"
		default:
			return "ctx:hit"
		}
	}
	if s.cacheMiss == "" || s.cacheMiss == "not_found" {
		return "ctx:rebuild"
	}
	return "ctx:miss(" + s.cacheMiss + ")"
}

// ExtractorFallbackActive reports whether extraction is using the main model.
func (s *Session) ExtractorFallbackActive() bool {
	return s.extractorFallback
}

// EmbeddingModelMissingActive reports whether embeddings are missing.
func (s *Session) EmbeddingModelMissingActive() bool {
	return s.missingEmbedding || s.embeddingModel == ""
}

// EmbeddingModel returns the currently selected embeddings model.
func (s *Session) EmbeddingModel() string {
	return s.embeddingModel
}

// Close archives the conversation and snapshots backups.
func (s *Session) Close(ctx context.Context) {
	if s.profileSlug != "" {
		s.stopBackupTicker()
		if err := backup.Snapshot(s.profileSlug); err != nil {
			s.logger.Errorf("backup snapshot failed: %v", err)
		}
	}

	s.waitForPendingNotes(4 * time.Second)
	_ = s.convRepo.Archive(ctx, s.conversationID)
}

type usageReportingEmbedder struct {
	embedder provider.Adapter
	onUsage  func(provider.Usage)
}

func (u *usageReportingEmbedder) Embed(ctx context.Context, req provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	if u == nil || u.embedder == nil {
		return nil, nil
	}
	resp, err := u.embedder.Embed(ctx, req)
	if err == nil && resp != nil && u.onUsage != nil {
		u.onUsage(resp.Usage)
	}
	return resp, err
}

func (s *Session) recordAuxUsage(usage provider.Usage) {
	s.stats.AddUsage(usage)
	if s.onStats != nil {
		s.onStats(s.stats)
	}
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

// sessionLogHook implements memory.CaptureLogHook and logs to the session logger.
type sessionLogHook struct {
	logger observe.Logger
}

func (h *sessionLogHook) CandidateScored(candidate memory.NoteCandidate) {
	h.logger.Infof("memory: candidate scored: value=%d", candidate.ValueScore.Total)
}

func (h *sessionLogHook) DuplicateDetected(candidate memory.NoteCandidate, existingID string) {
	h.logger.Infof("memory: duplicate detected: matched=%s", existingID)
}

func (h *sessionLogHook) NoteStored(candidate memory.NoteCandidate, noteID string) {
	h.logger.Infof("memory: note stored: id=%s", noteID)
}

func (h *sessionLogHook) NoteStorageFailed(candidate memory.NoteCandidate, err error) {
	h.logger.Errorf("memory: note storage failed: %v", err)
}

func (h *sessionLogHook) ExtractionPayloadRejected(reason string) {
	switch reason {
	case "has_new_info=false":
		h.logger.Infof("memory: extraction skipped: %s", reason)
	case "invalid json":
		h.logger.Infof("memory: extraction rejected: %s", reason)
	default:
		h.logger.Errorf("memory: extraction rejected: %s", reason)
	}
}

// injectDateTime prepends the current date and time to the user message
// so the LLM is aware of temporal context.
func injectDateTime(userMsg string) string {
	now := time.Now()
	dateTime := now.Format("Monday, January 2, 2006 at 3:04 PM MST")
	return fmt.Sprintf("[Current datetime: %s]\n\n%s", dateTime, userMsg)
}
