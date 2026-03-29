package chat

import (
	"context"
	"fmt"
	"time"

	"noto/internal/observe"
	"noto/internal/provider"
	"noto/internal/store"
)

// Pipeline executes a single chat turn: persists the user message, calls the provider,
// persists the assistant response, and returns the response text.
type Pipeline struct {
	convRepo *store.ConversationRepo
	msgRepo  *store.MessageRepo
	adapter  provider.Adapter
	logger   observe.Logger
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

func newMsgID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
