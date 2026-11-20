package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"noto/internal/chat"
	"noto/internal/provider"
	"noto/internal/store"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	maxRequestIDs   = 128
	sendTimeout     = 5 * time.Minute
)

var (
	// ErrProviderSetupRequired indicates that browser chat is unavailable until a provider is configured.
	ErrProviderSetupRequired = errors.New("web: provider setup required")
	// ErrSendInProgress indicates that another turn is currently mutating the chat session.
	ErrSendInProgress = errors.New("web: another message is in progress")
	// ErrRequestIDConflict indicates that an idempotency key was reused for different content.
	ErrRequestIDConflict = errors.New("web: request ID reused with different content")
	// ErrServiceStopped indicates that the web service is shutting down.
	ErrServiceStopped = errors.New("web: service is stopping")
	requestIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// ChatSession is the subset of the shared chat session used by the web layer.
type ChatSession interface {
	Send(context.Context, string) (*chat.SendResult, error)
	PendingNotes() int
}

type sendRecord struct {
	content string
	done    chan struct{}
	result  SendResponseDTO
	err     error
}

type sendRequestKey struct {
	profileSlug string
	requestID   string
}

// Service owns the active profile runtime and serializes its replacement against requests.
type Service struct {
	runtimeMu        sync.RWMutex
	runtime          *ProfileRuntime
	revision         uint64
	mu               sync.Mutex
	sending          bool
	switching        bool
	recoveryRequired bool
	active           *sendRecord
	stopped          bool
	requests         map[sendRequestKey]*sendRecord
	completedIDs     []sendRequestKey
	serviceCtx       context.Context
	serviceCancel    context.CancelFunc
}

// NewService creates an API service with one active profile runtime.
func NewService(db *store.DB, profile *store.Profile, session ChatSession, model string, setupRequired bool) *Service {
	serviceCtx, serviceCancel := context.WithCancel(context.Background())
	s := &Service{
		runtime: func() *ProfileRuntime {
			runtime := NewProfileRuntime(db, profile, session, model, setupRequired)
			return &runtime
		}(), revision: 1,
		requests: make(map[sendRequestKey]*sendRecord), serviceCtx: serviceCtx, serviceCancel: serviceCancel,
	}
	return s
}

// State returns browser-safe status for the active profile and session.
func (s *Service) State() StateDTO {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	runtime := s.runtime
	s.mu.Lock()
	sending, revision, recoveryRequired := s.sending, s.revision, s.recoveryRequired
	s.mu.Unlock()
	state := StateDTO{Revision: revision, Sending: sending, RecoveryRequired: recoveryRequired}
	if runtime == nil {
		return state
	}
	state.Model = runtime.Model
	state.SetupRequired = runtime.SetupRequired
	if runtime.Profile != nil {
		state.Profile.Name = runtime.Profile.Name
		state.Profile.Slug = runtime.Profile.Slug
	}
	if runtime.Session != nil {
		state.PendingNotes = runtime.Session.PendingNotes()
	}
	return state
}

// Models returns profile-specific choices while holding a read lease against runtime replacement.
func (s *Service) Models(ctx context.Context, backend SettingsBackend, slug string) (ProfileModelsDTO, error) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return backend.Models(ctx, slug)
}

// SaveSettings applies a settings selection atomically from the web service's perspective.
func (s *Service) SaveSettings(ctx context.Context, saver SettingsSaver, slug, model string, revision uint64) (StateDTO, error) {
	if saver == nil || slug == "" {
		return StateDTO{}, errors.New("web: invalid settings request")
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return StateDTO{}, ErrServiceStopped
	}
	if s.recoveryRequired {
		s.mu.Unlock()
		return StateDTO{}, ErrSettingsRecoveryRequired
	}
	if revision != s.revision {
		s.mu.Unlock()
		return StateDTO{}, ErrSettingsRevisionConflict
	}
	if s.sending || s.switching {
		s.mu.Unlock()
		return StateDTO{}, ErrSettingsBusy
	}
	s.switching = true
	current := s.runtime
	s.mu.Unlock()

	if current == nil {
		s.clearSwitching()
		return StateDTO{}, ErrServiceStopped
	}

	// Preparation can involve provider I/O and startup. A read lease keeps the
	// current runtime alive while allowing state/history reads to continue.
	s.runtimeMu.RLock()
	if s.runtime != current {
		s.runtimeMu.RUnlock()
		s.clearSwitching()
		return StateDTO{}, ErrSettingsRevisionConflict
	}
	if current.Session != nil && current.Session.PendingNotes() > 0 {
		s.runtimeMu.RUnlock()
		s.clearSwitching()
		return StateDTO{}, ErrSettingsBusy
	}
	prepared, err := saver.Prepare(ctx, *current, slug, model)
	s.runtimeMu.RUnlock()
	if err != nil {
		if errors.Is(err, ErrSettingsRecoveryRequired) {
			s.markRecoveryRequired()
		}
		s.clearSwitching()
		return StateDTO{}, err
	}
	if prepared == nil {
		s.clearSwitching()
		return StateDTO{}, errors.New("web: settings preparation returned no result")
	}
	if err := ctx.Err(); err != nil {
		prepared.Abort()
		s.clearSwitching()
		return StateDTO{}, err
	}

	s.runtimeMu.Lock()
	old := s.runtime
	s.mu.Lock()
	if s.stopped {
		s.switching = false
		s.mu.Unlock()
		s.runtimeMu.Unlock()
		prepared.Abort()
		return StateDTO{}, ErrServiceStopped
	}
	if revision != s.revision || old != current {
		s.switching = false
		s.mu.Unlock()
		s.runtimeMu.Unlock()
		prepared.Abort()
		return StateDTO{}, ErrSettingsRevisionConflict
	}
	if s.sending {
		s.switching = false
		s.mu.Unlock()
		s.runtimeMu.Unlock()
		prepared.Abort()
		return StateDTO{}, ErrSettingsBusy
	}
	s.mu.Unlock()
	if current.Session != nil && current.Session.PendingNotes() > 0 {
		s.runtimeMu.Unlock()
		prepared.Abort()
		s.clearSwitching()
		return StateDTO{}, ErrSettingsBusy
	}
	next, err := prepared.Commit(ctx)
	if err != nil {
		s.runtimeMu.Unlock()
		prepared.Abort()
		if errors.Is(err, ErrSettingsRecoveryRequired) {
			s.markRecoveryRequired()
		}
		s.clearSwitching()
		return StateDTO{}, err
	}
	s.runtime = &next
	s.mu.Lock()
	s.revision++
	s.switching = false
	s.mu.Unlock()

	if old != nil && old.Profile != nil && next.Profile != nil && old.Profile.Slug != next.Profile.Slug {
		retireCtx, cancelRetire := context.WithTimeout(context.Background(), 5*time.Second)
		if err := closeProfileRuntime(retireCtx, old); err != nil {
			log.Printf("noto web: retired profile runtime close failed error_type=%T", err)
		}
		cancelRetire()
	}
	s.runtimeMu.Unlock()
	return s.State(), nil
}

// SaveModelSettings applies a chat/embeddings/extractor selection using the same
// runtime replacement protocol as legacy chat-model-only settings saves.
func (s *Service) SaveModelSettings(ctx context.Context, saver ModelSettingsSaver, slug string, selection ModelSettingsSelection, revision uint64) (StateDTO, error) {
	if saver == nil {
		return StateDTO{}, errors.New("web: invalid settings request")
	}
	return s.SaveSettings(ctx, modelSettingsSaverAdapter{saver: saver, selection: selection}, slug, selection.Model, revision)
}

type modelSettingsSaverAdapter struct {
	saver     ModelSettingsSaver
	selection ModelSettingsSelection
}

func (a modelSettingsSaverAdapter) Prepare(ctx context.Context, current ProfileRuntime, slug, model string) (PreparedSettings, error) {
	a.selection.Model = model
	return a.saver.PrepareModels(ctx, current, slug, a.selection)
}

func (s *Service) clearSwitching() {
	s.mu.Lock()
	s.switching = false
	s.mu.Unlock()
}

func (s *Service) markRecoveryRequired() {
	s.mu.Lock()
	s.recoveryRequired = true
	s.mu.Unlock()
}

// CloseRuntime drains and closes the current profile resources after HTTP shutdown.
func (s *Service) CloseRuntime(ctx context.Context) error {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if s.runtime == nil {
		return nil
	}
	if err := closeProfileRuntime(ctx, s.runtime); err != nil {
		return err
	}
	s.runtime = nil
	return nil
}

func closeProfileRuntime(ctx context.Context, runtime *ProfileRuntime) error {
	if runtime == nil {
		return nil
	}
	if runtime.Session != nil {
		if drainer, ok := runtime.Session.(interface{ WaitForPendingNotes(context.Context) error }); ok {
			if err := drainer.WaitForPendingNotes(ctx); err != nil {
				return fmt.Errorf("web: wait for profile memory work: %w", err)
			}
		}
		if closer, ok := runtime.Session.(interface{ Close(context.Context) }); ok {
			closer.Close(ctx)
		}
	}
	if runtime.DB != nil {
		if err := runtime.DB.Close(); err != nil {
			return fmt.Errorf("web: close profile database: %w", err)
		}
	}
	return nil
}

// Messages returns one chronological page from this profile's persisted history.
func (s *Service) Messages(ctx context.Context, cursor *store.MessageHistoryCursor, limit int) (MessagePageDTO, error) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	runtime := s.runtime
	if runtime == nil || runtime.messageRepo == nil || runtime.Profile == nil {
		return MessagePageDTO{}, errors.New("web: message storage unavailable")
	}
	page, err := runtime.messageRepo.ListHistoryByProfile(ctx, runtime.Profile.ID, cursor, limit)
	if err != nil {
		return MessagePageDTO{}, err
	}
	out := MessagePageDTO{Items: make([]MessageDTO, 0, len(page.Messages))}
	for _, message := range page.Messages {
		out.Items = append(out.Items, MessageDTO{
			ID: message.ID, ConversationID: message.ConversationID,
			ConversationStartedAt: message.ConversationStartedAt,
			Role:                  string(message.Role), Content: message.Content, ContentHTML: RenderMarkdown(message.Content),
			CreatedAt: message.CreatedAt, Model: message.Model,
		})
	}
	if page.Next != nil {
		encoded, err := encodeCursor(page.Next)
		if err != nil {
			return MessagePageDTO{}, err
		}
		out.NextCursor = &encoded
	}
	return out, nil
}

// Memory returns profile notes or existing weekly/monthly summary artifacts.
func (s *Service) Memory(ctx context.Context, tab string, offset, limit int) (MemoryPageDTO, error) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	runtime := s.runtime
	if runtime == nil || runtime.Profile == nil || runtime.noteRepo == nil || runtime.summaryRepo == nil {
		return MemoryPageDTO{}, errors.New("web: memory storage unavailable")
	}
	out := MemoryPageDTO{Items: make([]MemoryDTO, 0)}
	switch tab {
	case "daily":
		notes, _, err := runtime.noteRepo.ListByProfilePaginated(ctx, runtime.Profile.ID, limit+1, offset)
		if err != nil {
			return MemoryPageDTO{}, err
		}
		more := len(notes) > limit
		if more {
			notes = notes[:limit]
		}
		for _, note := range notes {
			out.Items = append(out.Items, MemoryDTO{ID: note.ID, Content: note.Content, ContentHTML: RenderMarkdown(note.Content), CreatedAt: note.CreatedAt, Category: string(note.Category)})
		}
		if more {
			next := offset + limit
			out.NextOffset = &next
		}
	case store.SummaryTypeWeekly, store.SummaryTypeMonthly:
		summaries, _, err := runtime.summaryRepo.ListByProfileAndTypePaginated(ctx, runtime.Profile.ID, tab, limit+1, offset)
		if err != nil {
			return MemoryPageDTO{}, err
		}
		more := len(summaries) > limit
		if more {
			summaries = summaries[:limit]
		}
		for _, summary := range summaries {
			out.Items = append(out.Items, MemoryDTO{
				ID: summary.ID, Content: summary.Content, ContentHTML: RenderMarkdown(summary.Content),
				SummaryType: summary.SummaryType, PeriodKey: summary.PeriodKey,
				PeriodStart: summary.PeriodStart, PeriodEnd: summary.PeriodEnd, FreshnessState: summary.FreshnessState,
			})
		}
		if more {
			next := offset + limit
			out.NextOffset = &next
		}
	default:
		return MemoryPageDTO{}, fmt.Errorf("web: invalid memory tab %q", tab)
	}
	return out, nil
}

// Send serializes chat turns and deduplicates request IDs for this server process.
func (s *Service) Send(ctx context.Context, requestID, content string) (SendResponseDTO, error) {
	return s.send(ctx, requestID, content, "", false)
}

// SendForProfile rejects requests created against a stale profile before any session mutation.
func (s *Service) SendForProfile(ctx context.Context, requestID, content, profileSlug string) (SendResponseDTO, error) {
	return s.send(ctx, requestID, content, profileSlug, true)
}

func (s *Service) send(ctx context.Context, requestID, content, profileSlug string, checkProfile bool) (SendResponseDTO, error) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	runtime := s.runtime
	if !requestIDPattern.MatchString(requestID) || strings.TrimSpace(content) == "" {
		return SendResponseDTO{}, errors.New("web: invalid message request")
	}
	if checkProfile && (runtime == nil || runtime.Profile == nil || profileSlug == "" || profileSlug != runtime.Profile.Slug) {
		return SendResponseDTO{}, ErrProfileMismatch
	}
	if runtime == nil || runtime.SetupRequired || runtime.Session == nil {
		return SendResponseDTO{}, ErrProviderSetupRequired
	}
	activeProfileSlug := ""
	if runtime.Profile != nil {
		activeProfileSlug = runtime.Profile.Slug
	}
	requestKey := sendRequestKey{profileSlug: activeProfileSlug, requestID: requestID}
	if err := ctx.Err(); err != nil {
		return SendResponseDTO{}, err
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return SendResponseDTO{}, ErrServiceStopped
	}
	if s.recoveryRequired {
		s.mu.Unlock()
		return SendResponseDTO{}, ErrSettingsRecoveryRequired
	}
	if existing, ok := s.requests[requestKey]; ok {
		if existing.content != content {
			s.mu.Unlock()
			return SendResponseDTO{}, ErrRequestIDConflict
		}
		done := existing.done
		s.mu.Unlock()
		select {
		case <-done:
			return existing.result, existing.err
		case <-ctx.Done():
			return SendResponseDTO{}, ctx.Err()
		}
	}
	if s.sending || s.switching {
		s.mu.Unlock()
		return SendResponseDTO{}, ErrSendInProgress
	}
	record := &sendRecord{content: content, done: make(chan struct{})}
	s.requests[requestKey] = record
	s.sending = true
	s.active = record
	s.mu.Unlock()

	go s.runSend(runtime.Session, requestKey, requestID, content, record)
	select {
	case <-record.done:
		return record.result, record.err
	case <-ctx.Done():
		return SendResponseDTO{}, ctx.Err()
	}
}

func (s *Service) runSend(session ChatSession, requestKey sendRequestKey, requestID, content string, record *sendRecord) {
	ctx, cancel := context.WithTimeout(s.serviceCtx, sendTimeout)
	defer cancel()
	result, err := session.Send(ctx, content)
	if err == nil && result == nil {
		err = errors.New("web: chat session returned no result")
	}
	if err == nil {
		record.result = SendResponseDTO{RequestID: requestID, Reply: ReplyDTO{Content: result.Reply, ContentHTML: RenderMarkdown(result.Reply)}}
	} else {
		log.Printf("noto web: chat send failed category=%s error_type=%T", safeSendErrorCategory(err), err)
	}
	record.err = err

	s.mu.Lock()
	s.sending = false
	s.active = nil
	close(record.done)
	s.completedIDs = append(s.completedIDs, requestKey)
	for len(s.completedIDs) > maxRequestIDs {
		oldest := s.completedIDs[0]
		s.completedIDs = s.completedIDs[1:]
		delete(s.requests, oldest)
	}
	s.mu.Unlock()
}

func safeSendErrorCategory(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, provider.ErrInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(err, provider.ErrProviderUnavailable):
		return "provider_unavailable"
	default:
		return "chat_error"
	}
}

// Stop rejects new messages, cancels provider work, and waits for the active turn.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.stopped = true
	active := s.active
	cancel := s.serviceCancel
	s.mu.Unlock()
	cancel()
	if active == nil {
		return nil
	}
	select {
	case <-active.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("web: wait for active send: %w", ctx.Err())
	}
}

func encodeCursor(cursor *store.MessageHistoryCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("web: encode history cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeCursor(encoded string) (*store.MessageHistoryCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("web: invalid history cursor")
	}
	var cursor store.MessageHistoryCursor
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.CreatedAt.IsZero() || cursor.RowID <= 0 {
		return nil, errors.New("web: invalid history cursor")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("web: invalid history cursor")
	}
	return &cursor, nil
}
