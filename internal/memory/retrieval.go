package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"noto/internal/cache"
	"noto/internal/store"
	"noto/internal/vector"
)

// RetrievalContext is the assembled context payload for a chat turn.
type RetrievalContext struct {
	// SystemPrompt is the profile's system prompt.
	SystemPrompt string

	// MemoryBlock is the formatted block of relevant memory notes.
	MemoryBlock string

	// SessionSummary is the most recent session summary text.
	SessionSummary string

	// AssembledPrompt is the final combined system prompt with injected context.
	AssembledPrompt string

	// CacheHit indicates the assembled prompt was served from cache.
	CacheHit bool
	// CacheTier is the tier that served the response: l1|l2|none.
	CacheTier string
	// ServedStale indicates response was served from slightly stale cache.
	ServedStale bool
	// RevalidationStarted indicates async refresh was started.
	RevalidationStarted bool
	// MissReason provides best-effort cache miss reason for diagnostics.
	MissReason string
}

// CacheRepository manages cached assembled prompt payloads.
type CacheRepository interface {
	Get(ctx context.Context, profileID, cacheKey string) (*store.ContextCacheEntry, error)
	Upsert(ctx context.Context, e *store.ContextCacheEntry) error
	Invalidate(ctx context.Context, profileID, cacheKey string) error
}

// Retrieval assembles context for a chat turn from SQLite source-of-truth data.
type Retrieval struct {
	noteRepo             *store.MemoryNoteRepo
	sessionSummaryRepo   *store.SessionSummaryRepo
	memorySummaryRepo    *store.MemorySummaryRepo
	timelineSettingsRepo *store.TimelineSettingsRepo
	cacheRepo            CacheRepository
	vectorIndexPath      string
	warnFn               func(error)
	tokenBudget          int
	vectorIndex          vector.Index
	profileID            string
	embedder             vector.Embedder
	embeddingModel       string
	l1mu                 sync.RWMutex
	l1                   map[string]*store.ContextCacheEntry
	diag                 *cache.Diagnostics
}

// RetrievalOption configures Retrieval behavior.
type RetrievalOption func(*Retrieval)

// WithVectorIndexPath sets the vector index path for warning checks.
func WithVectorIndexPath(path string) RetrievalOption {
	return func(r *Retrieval) {
		r.vectorIndexPath = path
	}
}

// WithWarnFunc registers a warning hook for vector index issues.
func WithWarnFunc(fn func(error)) RetrievalOption {
	return func(r *Retrieval) {
		r.warnFn = fn
	}
}

// WithTokenBudget sets the token budget for selecting memory notes.
func WithTokenBudget(budget int) RetrievalOption {
	return func(r *Retrieval) {
		r.tokenBudget = budget
	}
}

// WithTimeline stores the repositories needed for time-layered retrieval.
func WithTimeline(settingsRepo *store.TimelineSettingsRepo, summaryRepo *store.MemorySummaryRepo) RetrievalOption {
	return func(r *Retrieval) {
		r.timelineSettingsRepo = settingsRepo
		r.memorySummaryRepo = summaryRepo
	}
}

// WithVectorRetrieval wires vector ranking into Retrieval.
func WithVectorRetrieval(index vector.Index, profileID string, embedder vector.Embedder, model string) RetrievalOption {
	return func(r *Retrieval) {
		r.vectorIndex = index
		r.profileID = profileID
		r.embedder = embedder
		r.embeddingModel = model
	}
}

// NewRetrieval creates a Retrieval service.
func NewRetrieval(noteRepo *store.MemoryNoteRepo, summaryRepo *store.SessionSummaryRepo, cacheRepo CacheRepository, opts ...RetrievalOption) *Retrieval {
	r := &Retrieval{noteRepo: noteRepo, sessionSummaryRepo: summaryRepo, cacheRepo: cacheRepo, l1: map[string]*store.ContextCacheEntry{}, diag: cache.NewDiagnostics()}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Assemble builds the RetrievalContext for a profile, reading from SQLite.
// It reuses cached context if available and valid.
func (r *Retrieval) Assemble(ctx context.Context, profileID, systemPrompt string) (*RetrievalContext, error) {
	start := time.Now()
	if err := r.checkVectorIndex(); err != nil && r.warnFn != nil {
		r.warnFn(err)
	}
	settings := store.DefaultTimelineSettings(profileID)
	if r.timelineSettingsRepo != nil {
		loaded, err := r.timelineSettingsRepo.GetOrDefault(ctx, profileID)
		if err != nil {
			return nil, fmt.Errorf("memory: get timeline settings: %w", err)
		}
		settings = loaded
	}

	notes, err := r.noteRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("memory: list notes: %w", err)
	}

	window, err := ComputeTimelineWindow(start, settings)
	if err != nil {
		return nil, fmt.Errorf("memory: compute timeline window: %w", err)
	}

	rawNotes, weeklySummaries, monthlySummaries := r.assembleTimelineLayers(ctx, profileID, notes, window)
	stateHash := hashTimelineState(rawNotes, weeklySummaries, monthlySummaries, settings)

	cacheKey := cacheKeyFor(profileID, systemPrompt, stateHash, r.embeddingModel)

	if entry, tier, servedStale, revalidating := r.getCached(ctx, profileID, cacheKey); entry != nil {
		var cachedCtx RetrievalContext
		if err := json.Unmarshal([]byte(entry.Payload), &cachedCtx); err == nil {
			cachedCtx.CacheHit = true
			cachedCtx.CacheTier = tier
			cachedCtx.ServedStale = servedStale
			cachedCtx.RevalidationStarted = revalidating
			cachedCtx.MissReason = ""
			r.diag.RecordHit(time.Since(start))
			return &cachedCtx, nil
		}
		_ = r.cacheRepo.Invalidate(ctx, profileID, cacheKey)
	}

	memoryBlock := BuildTimelineMemoryBlock(rawNotes, weeklySummaries, monthlySummaries)

	assembled := AssemblePrompt(systemPrompt, "", memoryBlock)

	ctxOut := &RetrievalContext{
		SystemPrompt:        systemPrompt,
		MemoryBlock:         memoryBlock,
		SessionSummary:      "",
		AssembledPrompt:     assembled,
		CacheHit:            false,
		CacheTier:           "none",
		ServedStale:         false,
		RevalidationStarted: false,
		MissReason:          "not_found",
	}

	if r.cacheRepo != nil {
		payload, _ := json.Marshal(ctxOut)
		expires := time.Now().Add(30 * 24 * time.Hour)
		sourceIDs, _ := json.Marshal(noteIDs(rawNotes))
		promptHash := sha256.Sum256([]byte(systemPrompt))
		entry := &store.ContextCacheEntry{
			ID:            fmt.Sprintf("cc-%x", time.Now().UnixNano()),
			ProfileID:     profileID,
			CacheKey:      cacheKey,
			Payload:       string(payload),
			SourceNoteIDs: string(sourceIDs),
			PromptVersion: fmt.Sprintf("prompt:%x", promptHash),
			StateVersion:  stateHash,
			CreatedAt:     time.Now().UTC(),
			ExpiresAt:     &expires,
		}
		_ = r.cacheRepo.Upsert(ctx, entry)
		r.putL1(cacheKey, entry)
	}
	if ctxOut.MissReason == "" {
		ctxOut.MissReason = "rebuild"
	}
	r.diag.RecordMiss(ctxOut.MissReason, time.Since(start))

	return ctxOut, nil
}

// BuildMemoryBlock formats notes into the memory block for prompts.
func BuildMemoryBlock(notes []*store.MemoryNote) string {
	if len(notes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Memory Notes\n")
	for _, n := range notes {
		fmt.Fprintf(&sb, "- [%s] %s\n", n.Category, n.Content)
	}
	return sb.String()
}

// BuildTimelineMemoryBlock formats raw notes, weekly summaries, and monthly summaries
// into distinct sections for prompt assembly.
func BuildTimelineMemoryBlock(rawNotes []*store.MemoryNote, weeklySummaries []*store.MemorySummary, monthlySummaries []*store.MemorySummary) string {
	var sb strings.Builder

	if len(rawNotes) > 0 {
		sb.WriteString("## Raw Notes\n")
		for _, n := range rawNotes {
			fmt.Fprintf(&sb, "- [%s] %s\n", n.Category, n.Content)
		}
	}
	if len(weeklySummaries) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("## Weekly Summaries\n")
		for _, s := range weeklySummaries {
			fmt.Fprintf(&sb, "- [%s] %s\n", s.PeriodKey, s.Content)
		}
	}
	if len(monthlySummaries) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("## Monthly Summaries\n")
		for _, s := range monthlySummaries {
			fmt.Fprintf(&sb, "- [%s] %s\n", s.PeriodKey, s.Content)
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// SelectNotesForContext orders notes by relevance (rankedIDs) and enforces token budget.
// If rankedIDs is empty, notes are assumed pre-sorted by importance then recency.
func SelectNotesForContext(notes []*store.MemoryNote, rankedIDs []string, budget int) []*store.MemoryNote {
	if len(notes) == 0 {
		return nil
	}
	if budget <= 0 {
		budget = 1500
	}

	ordered := notes
	if len(rankedIDs) > 0 {
		byID := make(map[string]*store.MemoryNote, len(notes))
		for _, n := range notes {
			byID[n.ID] = n
		}
		ordered = make([]*store.MemoryNote, 0, len(rankedIDs))
		for _, id := range rankedIDs {
			if note, ok := byID[id]; ok {
				ordered = append(ordered, note)
			}
		}
	} else {
		ordered = make([]*store.MemoryNote, len(notes))
		copy(ordered, notes)
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].Importance == ordered[j].Importance {
				return ordered[i].CreatedAt.After(ordered[j].CreatedAt)
			}
			return ordered[i].Importance > ordered[j].Importance
		})
	}

	selected := make([]*store.MemoryNote, 0, len(ordered))
	used := 0
	for _, note := range ordered {
		cost := estimateTokens(note.Content)
		if used+cost > budget {
			break
		}
		selected = append(selected, note)
		used += cost
	}
	return selected
}

func estimateTokens(content string) int {
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return 1
	}
	return len(fields)
}

// AssemblePrompt merges system prompt and memory block into the final prompt.
// sessionSummary is intentionally ignored because conversation summaries are no
// longer part of assembled memory context.
func AssemblePrompt(systemPrompt, sessionSummary, memoryBlock string) string {
	parts := []string{systemPrompt}
	if memoryBlock != "" {
		parts = append(parts, "\n"+memoryBlock)
	}
	return strings.Join(parts, "\n")
}

func cacheKeyFor(profileID, systemPrompt, stateHash, embeddingModel string) string {
	buf := fmt.Appendf(nil, "%s::%s::%s::%s", profileID, systemPrompt, stateHash, embeddingModel)
	hash := sha256.Sum256(buf)
	return fmt.Sprintf("ctx:%x", hash)
}

func (r *Retrieval) getCached(ctx context.Context, profileID, cacheKey string) (*store.ContextCacheEntry, string, bool, bool) {
	now := time.Now()
	if entry := r.getL1(cacheKey); entry != nil {
		if entry.ExpiresAt == nil || entry.ExpiresAt.After(now) {
			return entry, "l1", false, false
		}
		if entry.ExpiresAt.After(now.Add(-5 * time.Minute)) {
			go r.revalidate(profileID, cacheKey)
			return entry, "l1", true, true
		}
		r.deleteL1(cacheKey)
	}
	if r.cacheRepo != nil {
		cached, err := r.cacheRepo.Get(ctx, profileID, cacheKey)
		if err == nil && cached != nil {
			if cached.ExpiresAt == nil || cached.ExpiresAt.After(now) {
				r.putL1(cacheKey, cached)
				return cached, "l2", false, false
			}
			if cached.ExpiresAt.After(now.Add(-5 * time.Minute)) {
				r.putL1(cacheKey, cached)
				go r.revalidate(profileID, cacheKey)
				return cached, "l2", true, true
			}
			_ = r.cacheRepo.Invalidate(ctx, profileID, cacheKey)
		}
	}
	return nil, "none", false, false
}

func (r *Retrieval) revalidate(profileID, cacheKey string) {
	// non-blocking placeholder: ensure next request rebuilds fresh state
	r.deleteL1(cacheKey)
}

func (r *Retrieval) getL1(cacheKey string) *store.ContextCacheEntry {
	r.l1mu.RLock()
	defer r.l1mu.RUnlock()
	return r.l1[cacheKey]
}

func (r *Retrieval) putL1(cacheKey string, e *store.ContextCacheEntry) {
	r.l1mu.Lock()
	defer r.l1mu.Unlock()
	r.l1[cacheKey] = e
}

func (r *Retrieval) deleteL1(cacheKey string) {
	r.l1mu.Lock()
	defer r.l1mu.Unlock()
	delete(r.l1, cacheKey)
}

// CacheDiagnostics returns aggregated cache metrics for retrieval behavior.
func (r *Retrieval) CacheDiagnostics() cache.Snapshot {
	if r.diag == nil {
		return cache.Snapshot{}
	}
	return r.diag.Snapshot()
}

// hashNoteIDs creates a stable hash of note IDs for cache key generation.
// Sorts IDs first to ensure consistent hashing regardless of order.
func hashNoteIDs(ids []string) string {
	sort.Strings(ids)
	h := sha256.New()
	for _, id := range ids {
		h.Write([]byte(id))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func noteIDs(notes []*store.MemoryNote) []string {
	ids := make([]string, 0, len(notes))
	for _, note := range notes {
		ids = append(ids, note.ID)
	}
	return ids
}

func (r *Retrieval) checkVectorIndex() error {
	if r.vectorIndexPath == "" {
		return nil
	}
	info, err := os.Stat(r.vectorIndexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return vector.ErrIndexNotFound
		}
		return vector.ErrIndexCorrupted
	}
	if info.Size() == 0 {
		return vector.ErrIndexCorrupted
	}
	return nil
}

func hashTimelineState(rawNotes []*store.MemoryNote, weeklySummaries []*store.MemorySummary, monthlySummaries []*store.MemorySummary, settings *store.TimelineSettings) string {
	parts := make([]string, 0, len(rawNotes)+len(weeklySummaries)+len(monthlySummaries)+3)
	parts = append(parts, fmt.Sprintf("raw:%d", settings.RawNoteDays))
	parts = append(parts, fmt.Sprintf("weekly:%d", settings.WeeklySummaryWeeks))
	parts = append(parts, fmt.Sprintf("monthly:%d", settings.MonthlySummaryMonths))
	for _, n := range rawNotes {
		parts = append(parts, "n:"+n.ID)
	}
	for _, s := range weeklySummaries {
		parts = append(parts, "w:"+s.ID+":"+s.FreshnessState)
	}
	for _, s := range monthlySummaries {
		parts = append(parts, "m:"+s.ID+":"+s.FreshnessState)
	}
	return hashNoteIDs(parts)
}

// BuildTimelineStateHashForTest exposes deterministic timeline-state hashing for tests.
func BuildTimelineStateHashForTest(rawNoteIDs []string, weeklyStates []string, monthlyStates []string, settings *store.TimelineSettings) string {
	rawNotes := make([]*store.MemoryNote, 0, len(rawNoteIDs))
	for _, id := range rawNoteIDs {
		rawNotes = append(rawNotes, &store.MemoryNote{ID: id})
	}
	weeklySummaries := make([]*store.MemorySummary, 0, len(weeklyStates))
	for _, state := range weeklyStates {
		parts := strings.SplitN(state, ":", 2)
		freshness := ""
		if len(parts) > 1 {
			freshness = parts[1]
		}
		weeklySummaries = append(weeklySummaries, &store.MemorySummary{ID: parts[0], FreshnessState: freshness})
	}
	monthlySummaries := make([]*store.MemorySummary, 0, len(monthlyStates))
	for _, state := range monthlyStates {
		parts := strings.SplitN(state, ":", 2)
		freshness := ""
		if len(parts) > 1 {
			freshness = parts[1]
		}
		monthlySummaries = append(monthlySummaries, &store.MemorySummary{ID: parts[0], FreshnessState: freshness})
	}
	return hashTimelineState(rawNotes, weeklySummaries, monthlySummaries, settings)
}

func (r *Retrieval) assembleTimelineLayers(ctx context.Context, profileID string, notes []*store.MemoryNote, window TimelineWindow) ([]*store.MemoryNote, []*store.MemorySummary, []*store.MemorySummary) {
	rawNotes := filterNotesInRange(notes, window.RawStart, window.RawEnd)

	weeklySummaries := make([]*store.MemorySummary, 0)
	monthlySummaries := make([]*store.MemorySummary, 0)
	if r.memorySummaryRepo == nil {
		return rawNotes, weeklySummaries, monthlySummaries
	}

	weekly, err := r.memorySummaryRepo.ListByProfileAndType(ctx, profileID, store.SummaryTypeWeekly)
	if err == nil {
		for _, s := range weekly {
			if !s.PeriodStart.Before(window.WeeklyStart) && s.PeriodStart.Before(window.WeeklyEnd) && s.FreshnessState == store.SummaryFresh {
				weeklySummaries = append(weeklySummaries, s)
			}
		}
	}
	monthly, err := r.memorySummaryRepo.ListByProfileAndType(ctx, profileID, store.SummaryTypeMonthly)
	if err == nil {
		for _, s := range monthly {
			if !s.PeriodStart.Before(window.MonthlyStart) {
				continue
			}
			if window.MonthlyCutoff != nil && s.PeriodStart.Before(*window.MonthlyCutoff) {
				continue
			}
			if s.FreshnessState == store.SummaryFresh {
				monthlySummaries = append(monthlySummaries, s)
			}
		}
	}

	if len(weeklySummaries) == 0 && !window.WeeklyStart.Equal(window.WeeklyEnd) {
		rawNotes = append(rawNotes, filterNotesInRange(notes, window.WeeklyStart, window.WeeklyEnd)...)
	}
	if len(monthlySummaries) == 0 {
		monthlyEnd := window.MonthlyStart
		monthlyStart := time.Time{}
		if window.MonthlyCutoff != nil {
			monthlyStart = *window.MonthlyCutoff
		}
		if !monthlyEnd.IsZero() {
			rawNotes = append(rawNotes, filterNotesInRange(notes, monthlyStart, monthlyEnd)...)
		}
	}

	sort.Slice(rawNotes, func(i, j int) bool { return rawNotes[i].CreatedAt.After(rawNotes[j].CreatedAt) })
	sort.Slice(weeklySummaries, func(i, j int) bool { return weeklySummaries[i].PeriodStart.After(weeklySummaries[j].PeriodStart) })
	sort.Slice(monthlySummaries, func(i, j int) bool { return monthlySummaries[i].PeriodStart.After(monthlySummaries[j].PeriodStart) })
	return dedupeNotes(rawNotes), weeklySummaries, monthlySummaries
}

func filterNotesInRange(notes []*store.MemoryNote, start, end time.Time) []*store.MemoryNote {
	out := make([]*store.MemoryNote, 0)
	for _, n := range notes {
		if (n.CreatedAt.Equal(start) || n.CreatedAt.After(start)) && n.CreatedAt.Before(end) {
			out = append(out, n)
		}
	}
	return out
}

func dedupeNotes(notes []*store.MemoryNote) []*store.MemoryNote {
	seen := make(map[string]struct{}, len(notes))
	out := make([]*store.MemoryNote, 0, len(notes))
	for _, n := range notes {
		if _, ok := seen[n.ID]; ok {
			continue
		}
		seen[n.ID] = struct{}{}
		out = append(out, n)
	}
	return out
}
