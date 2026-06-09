package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"noto/internal/memory"
	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
)

// Pipeline executes a single chat turn: persists the user message, calls the provider,
// persists the assistant response, and returns the response text.
type Pipeline struct {
	convRepo     *store.ConversationRepo
	msgRepo      *store.MessageRepo
	noteRepo     *store.MemoryNoteRepo
	summaryRepo  *store.MemorySummaryRepo
	adapter      provider.Adapter
	logger       observe.Logger
	toolsEnabled bool
}

// NewPipeline creates a chat Pipeline.
func NewPipeline(
	convRepo *store.ConversationRepo,
	msgRepo *store.MessageRepo,
	adapter provider.Adapter,
	logger observe.Logger,
) *Pipeline {
	return &Pipeline{
		convRepo: convRepo,
		msgRepo:  msgRepo,
		adapter:  adapter,
		logger:   logger,
	}
}

// WithMemorySearchTools wires memory repositories into the pipeline.
func (p *Pipeline) WithMemorySearchTools(noteRepo *store.MemoryNoteRepo, summaryRepo *store.MemorySummaryRepo) *Pipeline {
	p.noteRepo = noteRepo
	p.summaryRepo = summaryRepo
	return p
}

// WithToolsEnabled gates whether tools are exposed to the provider.
func (p *Pipeline) WithToolsEnabled(enabled bool) *Pipeline {
	p.toolsEnabled = enabled
	return p
}

// TurnInput is the input for a single chat turn.
type TurnInput struct {
	ConversationID string
	ProfileID      string
	UserContent    string
	SystemPrompt   string
	// PriorMessages are the messages already in the conversation (for context window).
	PriorMessages []*store.Message
}

// TurnOutput is the result of a single chat turn.
type TurnOutput struct {
	AssistantContent string
	UserMessageID    string
	AssistantMsgID   string
	LatencyMs        int64
}

// Execute performs a single chat turn.
func (p *Pipeline) Execute(ctx context.Context, input TurnInput) (*TurnOutput, error) {
	start := time.Now()

	// Persist user message.
	userMsg := &store.Message{
		ID:             newMsgID(),
		ConversationID: input.ConversationID,
		Role:           store.RoleUser,
		Content:        input.UserContent,
		Provider:       p.adapter.ProviderType(),
	}
	if err := p.msgRepo.Create(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("chat: persist user message: %w", err)
	}

	// Build the completion request.
	var msgs []provider.Message
	if input.SystemPrompt != "" {
		msgs = append(msgs, provider.Message{Role: "system", Content: input.SystemPrompt})
	}
	for _, m := range input.PriorMessages {
		msgs = append(msgs, provider.Message{Role: string(m.Role), Content: m.Content})
	}
	msgs = append(msgs, provider.Message{Role: "user", Content: input.UserContent})

	req := provider.CompletionRequest{
		Messages:    msgs,
		Temperature: 0.7,
	}
	if p.toolsEnabled {
		req.Tools = []provider.ToolDefinition{
			{Type: "function", Name: "search_memory_keywords", Description: "Search memory by keyword", Parameters: map[string]any{"type": "object"}},
			{Type: "function", Name: "search_memory_time_range", Description: "Search memory by time range", Parameters: map[string]any{"type": "object"}},
		}
	}

	resp, err := p.adapter.Complete(ctx, req)
	if err != nil {
		p.logger.Emit(observe.Event{
			EventType: observe.EventProviderCall,
			ProfileID: input.ProfileID,
			Status:    observe.StatusFailure,
			Metadata:  map[string]any{"error": err.Error()},
		})
		return nil, fmt.Errorf("chat: provider call failed: %w", err)
	}

	if len(resp.ToolCalls) > 0 && p.toolsEnabled {
		followup := append([]provider.Message{}, req.Messages...)
		for _, call := range resp.ToolCalls {
			followup = append(followup, provider.Message{Role: "assistant", Content: call.Name, ToolCallID: call.CallID})
			toolResult := p.executeToolCall(ctx, input.ProfileID, call)
			followup = append(followup, provider.Message{Role: "tool", Content: toolResult, ToolCallID: call.CallID})
		}
		resp, err = p.adapter.Complete(ctx, provider.CompletionRequest{Model: req.Model, Messages: followup, Temperature: req.Temperature, Tools: req.Tools})
		if err != nil {
			return nil, fmt.Errorf("chat: provider follow-up failed: %w", err)
		}
	}

	// Persist assistant message.
	assistantMsg := &store.Message{
		ID:             newMsgID(),
		ConversationID: input.ConversationID,
		Role:           store.RoleAssistant,
		Content:        resp.Content,
		Provider:       p.adapter.ProviderType(),
		Model:          resp.Model,
	}
	if err := p.msgRepo.Create(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("chat: persist assistant message: %w", err)
	}

	latency := time.Since(start).Milliseconds()
	p.logger.Emit(observe.Event{
		EventType: observe.EventProviderCall,
		ProfileID: input.ProfileID,
		Status:    observe.StatusSuccess,
		LatencyMs: &latency,
	})

	return &TurnOutput{
		AssistantContent: resp.Content,
		UserMessageID:    userMsg.ID,
		AssistantMsgID:   assistantMsg.ID,
		LatencyMs:        latency,
	}, nil
}

func (p *Pipeline) executeToolCall(ctx context.Context, profileID string, call provider.ToolCall) string {
	switch call.Name {
	case "search_memory_keywords":
		if p.noteRepo == nil {
			return "[]"
		}
		var args struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(call.Arguments), &args)
		exec := memory.NewKeywordSearchTool(func(ctx context.Context) ([]*store.MemoryNote, error) {
			return p.noteRepo.ListByProfile(ctx, profileID)
		})
		results, _ := exec.Execute(ctx, memory.KeywordSearchInput{Query: args.Query, Limit: args.Limit})
		b, _ := json.Marshal(results)
		return string(b)
	case "search_memory_time_range":
		var args struct {
			StartTime time.Time `json:"start_time"`
			EndTime   time.Time `json:"end_time"`
			Limit     int       `json:"limit"`
		}
		_ = json.Unmarshal([]byte(call.Arguments), &args)
		exec := memory.NewTimeRangeSearchTool(
			func(ctx context.Context) ([]*store.MemoryNote, error) {
				if p.noteRepo == nil {
					return nil, nil
				}
				return p.noteRepo.ListByProfile(ctx, profileID)
			},
			func(ctx context.Context) ([]*store.MemorySummary, error) {
				if p.summaryRepo == nil {
					return nil, nil
				}
				weekly, _ := p.summaryRepo.ListByProfileAndType(ctx, profileID, store.SummaryTypeWeekly)
				monthly, _ := p.summaryRepo.ListByProfileAndType(ctx, profileID, store.SummaryTypeMonthly)
				return append(weekly, monthly...), nil
			},
		)
		results, _ := exec.Execute(ctx, memory.TimeRangeSearchInput{StartTime: args.StartTime, EndTime: args.EndTime, Limit: args.Limit})
		b, _ := json.Marshal(results)
		return string(b)
	default:
		return "[]"
	}
}

func newMsgID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
