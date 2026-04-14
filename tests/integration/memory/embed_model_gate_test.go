package integration

import (
	"context"
	"testing"

	"noto/internal/chat"
	"noto/internal/observe"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/store"
)

type stubAdapter struct{}

func (s *stubAdapter) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{Content: "ok", Model: "m"}, nil
}
func (s *stubAdapter) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{Embedding: []float32{0.1, 0.2}, Model: "m"}, nil
}
func (s *stubAdapter) ProviderType() string { return "stub" }

func TestEmbeddingsModelRequiredForRetrieval(t *testing.T) {
	ctx := context.Background()
	db, closeDB := tempDB(t)
	defer closeDB()

	svc := profile.NewService(store.NewProfileRepo(db))
	p, _ := svc.Create(ctx, "Embed Gate")

	convRepo := store.NewConversationRepo(db)
	msgRepo := store.NewMessageRepo(db)
	noteRepo := store.NewMemoryNoteRepo(db)
	summaryRepo := store.NewSessionSummaryRepo(db)

	sess, err := chat.NewSession(
		ctx,
		p.ID,
		p.Slug,
		"system",
		db,
		convRepo,
		msgRepo,
		noteRepo,
		summaryRepo,
		&stubAdapter{},
		&stubAdapter{},
		observe.NewNoopLogger(),
		func(int, int) {},
		func() {},
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if !sess.EmbeddingModelMissingActive() {
		t.Fatalf("expected missing embedding model")
	}

}
