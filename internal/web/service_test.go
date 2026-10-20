package web

import (
	"context"
	"testing"
	"time"

	"noto/internal/chat"
	"noto/internal/store"
)

func TestServiceStopCancelsAndWaitsForSend(t *testing.T) {
	session := &stopBlockingSession{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(nil, &store.Profile{ID: "p1", Name: "Test"}, session, "model", false)
	sendDone := make(chan error, 1)
	go func() {
		_, err := service.Send(context.Background(), "stop-test", "hello")
		sendDone <- err
	}()
	select {
	case <-session.started:
	case <-time.After(2 * time.Second):
		t.Fatal("send did not start")
	}

	stopDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		stopDone <- service.Stop(ctx)
	}()
	select {
	case err := <-stopDone:
		t.Fatalf("Stop returned before active send finished: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(session.release)
	select {
	case err := <-stopDone:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not wait for send completion")
	}
	if _, err := service.Send(context.Background(), "after-stop", "hello"); err != ErrServiceStopped {
		t.Fatalf("Send after Stop error = %v, want ErrServiceStopped", err)
	}
	if err := <-sendDone; err != nil {
		t.Fatalf("completed Send: %v", err)
	}
}

type stopBlockingSession struct {
	started chan struct{}
	release chan struct{}
}

func (s *stopBlockingSession) PendingNotes() int { return 0 }
func (s *stopBlockingSession) Send(context.Context, string) (*chat.SendResult, error) {
	close(s.started)
	<-s.release
	return &chat.SendResult{Reply: "complete"}, nil
}
