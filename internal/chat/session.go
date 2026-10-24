package chat

import (
	"context"
	"encoding/json"
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
	"noto/internal/reference"
	"noto/internal/store"
	"noto/internal/vector"
	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

const (
	// recentHistoryMessages is the number of messages from the most recent
	// previous conversation to prepend to context on session start.
	recentHistoryMessages = 20

	// maxToolRounds bounds tool-call follow-up rounds per chat turn so a
	// model that keeps emitting tool calls cannot loop forever.
	maxToolRounds = 5
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

	convRepo            *store.ConversationRepo
	msgRepo             *store.MessageRepo
	noteRepo            *store.MemoryNoteRepo
	cacheRepo           *store.ContextCacheRepo
	adapter             provider.Adapter
	retrieval           *memory.Retrieval
	extractor           *memory.Extractor
	extractorAdapter    provider.Adapter
	logger              observe.Logger
	baseSystemPrompt    string
	db                  *store.DB
	vectorIndexPath     string
	vectorIndex         vector.Index
	extractorFallback   bool
	embeddingModel      string
	missingEmbedding    bool
	toolsEnabled        bool
	modelContextMax     int
	referenceContextMax int
	references          *reference.Service
	referenceWarning    bool
	adapterMu           sync.RWMutex
	statsMu             sync.Mutex

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
	if profileSlug != "" && db != nil {
		filesDir, pathErr := config.ProfileFilesDir(profileSlug)
		if pathErr != nil {
			s.referenceWarning = true
			logger.Infof("reference files directory unavailable: %v", pathErr)
		} else {
			refs, refErr := reference.NewService(reference.Options{
				Root: filesDir, ProfileID: profileID, Repo: store.NewReferenceFileRepo(db),
				Model:   func() reference.Model { return s.referenceModel() },
				OnUsage: s.recordAuxUsage,
			})
			if refErr != nil {
				s.referenceWarning = true
				logger.Infof("reference files unavailable: %v", refErr)
			} else {
				s.references = refs
				diagnostics, scanErr := refs.Scan(ctx)
				if scanErr != nil || len(diagnostics) > 0 {
					s.referenceWarning = true
					logger.Infof("reference file scan completed with issues: files=%d err=%v", len(diagnostics), scanErr)
				}
			}
		}
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
	var referenceTurn reference.Turn
	if s.references != nil {
		baseMessages := []provider.Message{{Role: "system", Content: systemPrompt}}
		recentUsers := make([]string, 0, 4)
		for i, message := range s.history {
			if i == len(s.history)-1 && message.Role == store.RoleUser {
				continue
			}
			baseMessages = append(baseMessages, provider.Message{Role: string(message.Role), Content: message.Content})
			if message.Role == store.RoleUser {
				recentUsers = append(recentUsers, message.Content)
			}
		}
		prepared, prepareErr := s.references.Prepare(ctx, reference.Input{
			UserMessage: userMsg, RecentUserMessages: recentUsers,
			BaseMessages: baseMessages, ContextMax: s.modelContextMaxSnapshot(),
		})
		referenceTurn = prepared
		if prepareErr != nil {
			if ctx.Err() != nil {
				return nil, prepareErr
			}
			s.logger.Infof("reference preparation failed: %v", prepareErr)
			referenceTurn.Degraded = true
			referenceTurn.Diagnostics = append(referenceTurn.Diagnostics, reference.Diagnostic{Code: "reference_unavailable", Message: "reference files were not provided"})
		}
	}
	if s.references != nil {
		s.setReferenceWarning(referenceTurn.Degraded || len(referenceTurn.Diagnostics) > 0)
	}
	if s.onContextStatus != nil {
		s.onContextStatus(s.CacheStatus())
	}
	msgs := buildReferenceMessages(systemPrompt, referenceTurn.Catalog, s.history, referenceTurn.Attachments, injectedMsg, referenceNotice(referenceTurn.Diagnostics))

	req := provider.CompletionRequest{
		Messages:    msgs,
		Temperature: 0.7,
	}
	if s.toolsEnabled {
		req.Tools = chatToolDefinitions()
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
		pipeline := NewPipeline(s.convRepo, s.msgRepo, s.adapter, s.logger).WithMemorySearchTools(s.noteRepo, store.NewMemorySummaryRepo(s.db)).WithToolsEnabled(true).WithSaveNoteHandler(func(ctx context.Context, call provider.ToolCall) string {
			return s.handleSaveNoteCall(ctx, call, []string{userMsgRec.ID})
		})
		rounds := 0
		for len(resp.ToolCalls) > 0 && rounds < maxToolRounds {
			rounds++
			for _, call := range resp.ToolCalls {
				s.logger.Infof("tools: executing tool call name=%s call_id=%s args=%s", call.Name, call.CallID, call.Arguments)
				followup = append(followup, provider.Message{Role: "assistant", Content: call.Name, ToolCallID: call.CallID, ToolCallName: call.Name, ToolCallArguments: call.Arguments, ToolID: call.ID})
				result := pipeline.executeToolCall(ctx, s.profileID, call)
				s.logger.Infof("tools: tool call completed name=%s call_id=%s", call.Name, call.CallID)
				followup = append(followup, provider.Message{Role: "tool", Content: result, ToolCallID: call.CallID})
			}
			resp, err = s.adapter.Complete(ctx, provider.CompletionRequest{Model: req.Model, Messages: followup, Temperature: req.Temperature, Tools: req.Tools})
			if err != nil {
				s.logger.Errorf("provider follow-up call failed (error type=%T)", err)
				break
			}
			s.logger.Infof("tools: follow-up provider call completed after tool execution (round %d/%d)", rounds, maxToolRounds)
		}
		if err == nil && len(resp.ToolCalls) > 0 {
			s.logger.Infof("tools: round limit reached (%d); stopping tool loop with final response", maxToolRounds)
		}
	}
	if err != nil {
		s.logger.Errorf("provider call failed (error type=%T)", err)
		return nil, fmt.Errorf("session: provider call: %w", err)
	}

	latency := time.Since(start).Milliseconds()
	influenced := referenceInfluenced(s.history, referenceTurn.Catalog, referenceTurn.Attachments)

	// Persist assistant message.
	asstMsgRec := &store.Message{
		ID:                  fmt.Sprintf("msg-%x", time.Now().UnixNano()),
		ConversationID:      s.conversationID,
		Role:                store.RoleAssistant,
		Content:             resp.Content,
		Provider:            s.adapter.ProviderType(),
		Model:               resp.Model,
		ReferenceInfluenced: influenced,
	}
	if err := s.msgRepo.Create(ctx, asstMsgRec); err != nil {
		return nil, fmt.Errorf("session: persist assistant message: %w", err)
	}
	s.history = append(s.history, asstMsgRec)

	if resp.ContextMax == 0 {
		if contextMax := s.modelContextMaxSnapshot(); contextMax > 0 {
			resp.ContextMax = contextMax
		}
	}

	// Accumulate token/cost stats.
	s.statsMu.Lock()
	s.stats.AddCompletion(resp)
	stats := s.stats
	s.statsMu.Unlock()
	if s.onStats != nil {
		s.onStats(stats)
	}

	// Capture immutable turn inputs before launching extraction, so later turns cannot change provenance or source IDs.
	sourceIDs, extractionUser, extractionAssistant := memoryExtractionInput(userMsgRec.ID, asstMsgRec.ID, userMsg, resp.Content, influenced)
	s.markNotesPending()
	go s.extractAsync(extractionUser, extractionAssistant, sourceIDs)

	return &SendResult{Reply: resp.Content, LatencyMs: latency}, nil
}

// Stats returns a snapshot of current session usage stats.
func (s *Session) Stats() provider.Stats {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.stats
}

// PendingNotes returns the number of asynchronous note extractions in flight.
func (s *Session) PendingNotes() int {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return s.pendingNotes
}

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

func (s *Session) extractAsync(userMsg, assistantMsg string, sourceIDs []string) {
	if s.onNotesSaving != nil {
		s.onNotesSaving()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	extractor := s.extractor
	s.adapterMu.RLock()
	extractorAdapter := s.extractorAdapter
	s.adapterMu.RUnlock()
	if extractorAdapter != nil {
		extractor = memory.NewExtractor(s.noteRepo, extractorAdapter, s.cacheRepo)
	} else if s.adapter != nil {
		extractor = memory.NewExtractor(s.noteRepo, s.adapter, s.cacheRepo)
	}
	if deduper := s.buildVectorDeduper(); deduper != nil {
		extractor.WithDeduper(deduper)
	}
	extractor.WithLogHook(&sessionLogHook{logger: s.logger})
	extractor.WithUsageHook(s.recordAuxUsage)
	if s.db != nil {
		if settings, err := store.NewTimelineSettingsRepo(s.db).GetOrDefault(ctx, s.profileID); err == nil {
			extractor.WithNoteMaxAgeDays(settings.DedupMaxAgeDays)
		}
	}
	result, err := extractor.ExtractTurn(ctx, s.profileID, s.profileSlug, s.conversationID, sourceIDs, userMsg, assistantMsg)
	if err != nil {
		s.logger.Errorf("memory extraction failed (error type=%T)", err)
		s.markNotesDone(0)
		return
	}
	s.logger.Infof("extraction result: notes=%d, updated=%d, merged=%d", len(result.Notes), result.Updated, result.Merged)

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

// buildVectorDeduper returns a vector deduper for note dedup, or nil when
// embeddings are unavailable. Shared by background extraction and save_note.
func (s *Session) buildVectorDeduper() vector.Deduper {
	s.adapterMu.RLock()
	extractorAdapter := s.extractorAdapter
	s.adapterMu.RUnlock()
	var embedder provider.Adapter
	if extractorAdapter != nil {
		embedder = extractorAdapter
	} else if s.adapter != nil {
		embedder = s.adapter
	}
	if embedder == nil || s.vectorIndex == nil || s.embeddingModel == "" {
		return nil
	}
	dedupEmbedder := &usageReportingEmbedder{embedder: embedder, onUsage: s.recordAuxUsage}
	return vector.NewVectorDeduper(s.vectorIndex, s.profileID, dedupEmbedder, s.embeddingModel).WithWarnFunc(func(err error) {
		s.logger.Infof("vector dedup issue: %v", err)
	})
}

// handleSaveNoteCall executes a save_note tool call. It persists the note
// through the shared note-writing path, invalidates the retrieval cache, syncs
// the vector index, and notifies the footer via onNotes — mirroring the
// extractor's lifecycle. The note's profile/conversation binding is taken
// from the session, never from tool arguments.
func (s *Session) handleSaveNoteCall(ctx context.Context, call provider.ToolCall, sourceMessageIDs []string) string {
	var args struct {
		Content    string `json:"content"`
		Category   string `json:"category"`
		Importance int    `json:"importance"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		s.logger.Errorf("tools: save_note args decode failed call_id=%s err=%v raw=%s", call.CallID, err, call.Arguments)
		return marshalToolResult(map[string]any{"saved": false, "error": "invalid arguments"})
	}
	res, err := memory.SaveNoteExplicit(ctx, s.noteRepo, s.buildVectorDeduper(), &sessionLogHook{logger: s.logger}, s.profileID, s.conversationID,
		memory.SaveNoteInput{Content: args.Content, Category: args.Category, Importance: args.Importance}, sourceMessageIDs)
	if err != nil {
		s.logger.Errorf("tools: save_note failed call_id=%s err=%v", call.CallID, err)
		return marshalToolResult(map[string]any{"saved": false, "error": err.Error()})
	}
	if s.cacheRepo != nil {
		if err := s.cacheRepo.InvalidateAll(ctx, s.profileID); err != nil {
			s.logger.Errorf("tools: save_note cache invalidation failed: %v", err)
		}
	}
	if err := s.syncVectorIndex(ctx, []*store.MemoryNote{res.Note}); err != nil {
		s.logger.Errorf("tools: save_note vector sync failed: %v", err)
	}
	if s.onNotes != nil {
		if res.Updated {
			s.onNotes(0, 1)
		} else {
			s.onNotes(1, 0)
		}
	}
	s.logger.Infof("tools: save_note stored note_id=%s updated=%t", res.Note.ID, res.Updated)
	return marshalToolResult(map[string]any{"saved": true, "updated": res.Updated, "note_id": res.Note.ID})
}

func marshalToolResult(payload map[string]any) string {
	b, err := json.Marshal(payload)
	if err != nil {
		return `{"saved":false,"error":"tool result encoding failed"}`
	}
	return string(b)
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
	s.adapterMu.Lock()
	s.modelContextMax = contextMax
	fallback := s.extractorFallback
	if fallback {
		s.referenceContextMax = contextMax
	}
	s.adapterMu.Unlock()
	if contextMax > 0 {
		s.statsMu.Lock()
		s.stats.ContextMax = contextMax
		stats := s.stats
		s.statsMu.Unlock()
		if s.onStats != nil {
			s.onStats(stats)
		}
	}
}

// SetReferenceModelContextMax updates the known window for extractor/selector calls.
func (s *Session) SetReferenceModelContextMax(contextMax int) {
	s.adapterMu.Lock()
	s.referenceContextMax = contextMax
	s.adapterMu.Unlock()
}

// SetExtractorAdapter replaces the adapter used for extraction and reference-file decisions.
func (s *Session) SetExtractorAdapter(adapter provider.Adapter) {
	s.adapterMu.Lock()
	s.extractorAdapter = adapter
	s.extractorFallback = adapter == nil
	if adapter == nil {
		s.referenceContextMax = s.modelContextMax
	} else {
		s.referenceContextMax = 0
	}
	s.adapterMu.Unlock()
}

func (s *Session) referenceModel() reference.Model {
	s.adapterMu.RLock()
	defer s.adapterMu.RUnlock()
	adapter, contextMax := s.extractorAdapter, s.referenceContextMax
	if adapter == nil {
		adapter, contextMax = s.adapter, s.modelContextMax
	}
	return reference.Model{Adapter: adapter, ContextMax: contextMax}
}

func (s *Session) modelContextMaxSnapshot() int {
	s.adapterMu.RLock()
	defer s.adapterMu.RUnlock()
	return s.modelContextMax
}

func (s *Session) setReferenceWarning(warning bool) {
	s.referenceWarning = warning
	if s.onContextStatus != nil {
		s.onContextStatus(s.CacheStatus())
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
	s.adapterMu.RLock()
	adapter := s.extractorAdapter
	s.adapterMu.RUnlock()
	if adapter == nil {
		return
	}
	if a, ok := adapter.(interface{ SetModel(string) }); ok {
		a.SetModel(model)
	}
}

// CacheStatus returns a short label for the footer showing current context state.
func (s *Session) CacheStatus() string {
	status := ""
	switch {
	case s.cacheStale && s.cacheReval:
		status = "ctx:swr"
	case s.cacheHit:
		switch s.cacheTier {
		case "l1":
			status = "ctx:l1-hit"
		case "l2":
			status = "ctx:l2-hit"
		default:
			status = "ctx:hit"
		}
	case s.cacheMiss == "" || s.cacheMiss == "not_found":
		status = "ctx:rebuild"
	default:
		status = "ctx:miss(" + s.cacheMiss + ")"
	}
	if s.referenceWarning {
		return status + " files!"
	}
	return status
}

// ExtractorFallbackActive reports whether extraction is using the main model.
func (s *Session) ExtractorFallbackActive() bool {
	s.adapterMu.RLock()
	defer s.adapterMu.RUnlock()
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
	if s.references != nil {
		if err := s.references.Close(); err != nil {
			s.logger.Infof("reference service close failed: %v", err)
		}
		s.references = nil
	}
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
	s.statsMu.Lock()
	s.stats.AddUsage(usage)
	stats := s.stats
	s.statsMu.Unlock()
	if s.onStats != nil {
		s.onStats(stats)
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
