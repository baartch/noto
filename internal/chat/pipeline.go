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
			{Type: "function", Name: "search_memory_keywords", Description: "Search memory notes by keyword query. Results are objects with content, category, created_at, and importance.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Keyword or short phrase to search for in memory notes"}}, "required": []string{"query"}}},
			{Type: "function", Name: "search_memory_time_range", Description: "Search memory notes within a time range. Results are objects with content, category, created_at, and importance.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"start_time": map[string]any{"type": "string", "description": "Start of the time range in RFC3339 format"}, "end_time": map[string]any{"type": "string", "description": "End of the time range in RFC3339 format"}}, "required": []string{"start_time", "end_time"}}},
		}
		p.logger.Infof("tools: enabled for pipeline request with %d tool definitions", len(req.Tools))
	} else {
		p.logger.Infof("tools: disabled for pipeline request")
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

	p.logger.Infof("tools: provider returned %d tool call(s)", len(resp.ToolCalls))
	if len(resp.ToolCalls) > 0 && p.toolsEnabled {
		followup := append([]provider.Message{}, req.Messages...)
		for _, call := range resp.ToolCalls {
			p.logger.Infof("tools: executing tool call name=%s call_id=%s args=%s", call.Name, call.CallID, call.Arguments)
			followup = append(followup, provider.Message{Role: "assistant", Content: call.Name, ToolCallID: call.CallID, ToolCallName: call.Name, ToolCallArguments: call.Arguments, ToolID: call.ID})
			toolResult := p.executeToolCall(ctx, input.ProfileID, call)
			p.logger.Infof("tools: tool call completed name=%s call_id=%s", call.Name, call.CallID)
			followup = append(followup, provider.Message{Role: "tool", Content: toolResult, ToolCallID: call.CallID})
		}
		resp, err = p.adapter.Complete(ctx, provider.CompletionRequest{Model: req.Model, Messages: followup, Temperature: req.Temperature, Tools: req.Tools})
		if err != nil {
			return nil, fmt.Errorf("chat: provider follow-up failed: %w", err)
		}
		p.logger.Infof("tools: follow-up provider call completed after tool execution")
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
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			p.logger.Errorf("tools: keyword search args decode failed call_id=%s err=%v raw=%s", call.CallID, err, call.Arguments)
			return "[]"
		}
		exec := memory.NewKeywordSearchTool(func(ctx context.Context) ([]*store.MemoryNote, error) {
			return p.noteRepo.ListByProfile(ctx, profileID)
		})
		results, err := exec.Execute(ctx, memory.KeywordSearchInput{Query: args.Query, Limit: args.Limit})
		if err != nil {
			p.logger.Errorf("tools: keyword search failed call_id=%s err=%v query=%q limit=%d", call.CallID, err, args.Query, args.Limit)
			return "[]"
		}
		if results == nil {
			results = []memory.SearchResultItem{}
		}
		b, _ := json.Marshal(results)
		p.logger.Infof("tools: keyword search returned %d result(s) call_id=%s query=%q limit=%d", len(results), call.CallID, args.Query, args.Limit)
		return string(b)
	case "search_memory_time_range":
		var args struct {
			StartTime time.Time `json:"start_time"`
			EndTime   time.Time `json:"end_time"`
			Limit     int       `json:"limit"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			p.logger.Errorf("tools: time-range search args decode failed call_id=%s err=%v raw=%s", call.CallID, err, call.Arguments)
			return "[]"
		}
		exec := memory.NewTimeRangeSearchTool(
			func(ctx context.Context) ([]*store.MemoryNote, error) {
				if p.noteRepo == nil {
					return nil, nil
				}
				return p.noteRepo.ListByProfile(ctx, profileID)
			},
		)
		results, err := exec.Execute(ctx, memory.TimeRangeSearchInput{StartTime: args.StartTime, EndTime: args.EndTime, Limit: args.Limit})
		if err != nil {
			p.logger.Errorf("tools: time-range search failed call_id=%s err=%v start=%s end=%s limit=%d", call.CallID, err, args.StartTime.Format(time.RFC3339), args.EndTime.Format(time.RFC3339), args.Limit)
			return "[]"
		}
		if results == nil {
			results = []memory.SearchResultItem{}
		}
		b, _ := json.Marshal(results)
		p.logger.Infof("tools: time-range search returned %d result(s) call_id=%s start=%s end=%s limit=%d", len(results), call.CallID, args.StartTime.Format(time.RFC3339), args.EndTime.Format(time.RFC3339), args.Limit)
		return string(b)
	default:
		return "[]"
	}
}

func newMsgID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
