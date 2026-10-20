package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"noto/internal/chat"
	"noto/internal/store"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	maxRequestIDs   = 128
	sendTimeout     = 5 * time.Minute
)

var (
	ErrProviderSetupRequired = errors.New("web: provider setup required")
	ErrSendInProgress        = errors.New("web: another message is in progress")
	ErrRequestIDConflict     = errors.New("web: request ID reused with different content")
	ErrServiceStopped        = errors.New("web: service is stopping")
	requestIDPattern         = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
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

// Service is bound to one profile for its lifetime; callers cannot select a profile per request.
type Service struct {
	db            *store.DB
	profile       *store.Profile
	session       ChatSession
	model         string
	setupRequired bool

	messageRepo *store.MessageRepo
	noteRepo    *store.MemoryNoteRepo
	summaryRepo *store.MemorySummaryRepo

	mu            sync.Mutex
	sending       bool
	stopped       bool
	requests      map[string]*sendRecord
	completedIDs  []string
	serviceCtx    context.Context
	serviceCancel context.CancelFunc
}

func NewService(db *store.DB, profile *store.Profile, session ChatSession, model string, setupRequired bool) *Service {
	serviceCtx, serviceCancel := context.WithCancel(context.Background())
	s := &Service{
		db: db, session: session, model: model, setupRequired: setupRequired,
		requests: make(map[string]*sendRecord), serviceCtx: serviceCtx, serviceCancel: serviceCancel,
	}
	if profile != nil {
		profileCopy := *profile
		s.profile = &profileCopy
	}
	if db != nil {
		s.messageRepo = store.NewMessageRepo(db)
		s.noteRepo = store.NewMemoryNoteRepo(db)
		s.summaryRepo = store.NewMemorySummaryRepo(db)
	}
	return s
}

func (s *Service) State() StateDTO {
	s.mu.Lock()
	sending := s.sending
	s.mu.Unlock()
	state := StateDTO{Model: s.model, SetupRequired: s.setupRequired, Sending: sending}
	if s.profile != nil {
		state.Profile.Name = s.profile.Name
	}
	if s.session != nil {
		state.PendingNotes = s.session.PendingNotes()
	}
	return state
}

func (s *Service) Messages(ctx context.Context, cursor *store.MessageHistoryCursor, limit int) (MessagePageDTO, error) {
	if s.messageRepo == nil || s.profile == nil {
		return MessagePageDTO{}, errors.New("web: message storage unavailable")
	}
	page, err := s.messageRepo.ListHistoryByProfile(ctx, s.profile.ID, cursor, limit)
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

func (s *Service) Memory(ctx context.Context, tab string, offset, limit int) (MemoryPageDTO, error) {
	if s.profile == nil || s.noteRepo == nil || s.summaryRepo == nil {
		return MemoryPageDTO{}, errors.New("web: memory storage unavailable")
	}
	out := MemoryPageDTO{Items: make([]MemoryDTO, 0)}
	switch tab {
	case "daily":
		notes, _, err := s.noteRepo.ListByProfilePaginated(ctx, s.profile.ID, limit+1, offset)
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
		summaries, _, err := s.summaryRepo.ListByProfileAndTypePaginated(ctx, s.profile.ID, tab, limit+1, offset)
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

func (s *Service) Send(ctx context.Context, requestID, content string) (SendResponseDTO, error) {
	if !requestIDPattern.MatchString(requestID) || strings.TrimSpace(content) == "" {
		return SendResponseDTO{}, errors.New("web: invalid message request")
	}
	if s.setupRequired || s.session == nil {
		return SendResponseDTO{}, ErrProviderSetupRequired
	}
	if err := ctx.Err(); err != nil {
		return SendResponseDTO{}, err
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return SendResponseDTO{}, ErrServiceStopped
	}
	if existing, ok := s.requests[requestID]; ok {
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
	if s.sending {
		s.mu.Unlock()
		return SendResponseDTO{}, ErrSendInProgress
	}
	record := &sendRecord{content: content, done: make(chan struct{})}
	s.requests[requestID] = record
	s.sending = true
	s.mu.Unlock()

	go s.runSend(requestID, content, record)
	select {
	case <-record.done:
		return record.result, record.err
	case <-ctx.Done():
		return SendResponseDTO{}, ctx.Err()
	}
}

func (s *Service) runSend(requestID, content string, record *sendRecord) {
	ctx, cancel := context.WithTimeout(s.serviceCtx, sendTimeout)
	defer cancel()
	result, err := s.session.Send(ctx, content)
	if err == nil && result == nil {
		err = errors.New("web: chat session returned no result")
	}
	if err == nil {
		record.result = SendResponseDTO{RequestID: requestID, Reply: ReplyDTO{Content: result.Reply, ContentHTML: RenderMarkdown(result.Reply)}}
	}
	record.err = err

	s.mu.Lock()
	s.sending = false
	close(record.done)
	s.completedIDs = append(s.completedIDs, requestID)
	for len(s.completedIDs) > maxRequestIDs {
		oldest := s.completedIDs[0]
		s.completedIDs = s.completedIDs[1:]
		delete(s.requests, oldest)
	}
	s.mu.Unlock()
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
