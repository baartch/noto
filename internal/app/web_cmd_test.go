package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"noto/internal/chat"
	"noto/internal/config"
	"noto/internal/store"
	"noto/internal/web"
)

func TestWebCommandRegistered(t *testing.T) {
	command, _, err := RootCmd().Find([]string{"web"})
	if err != nil || command == nil || command.Use != "web" {
		t.Fatalf("RootCmd().Find(web) = (%v, %v)", command, err)
	}
}

func TestWebLoopbackOnly(t *testing.T) {
	listener, err := listenWeb(0)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("listener address = %v, want loopback", listener.Addr())
	}
}

func TestWebStartupPrintsActualPortAndServesReadOnlyWithoutProvider(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", appDir)
	t.Setenv("NOTO_API_KEY", "")
	output := newNotifyWriter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := webCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--port", "0"})
	cmd.SetOut(output)
	cmd.SetErr(output)
	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()
	line := output.waitFor(t, "Server running on port ")
	var port int
	if _, err := fmt.Sscanf(line, "Server running on port %d:", &port); err != nil || port < 1 || port > 65535 {
		t.Fatalf("startup output %q did not contain actual port: %v", line, err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	response, err := http.Get(baseURL + "/api/state")
	if err != nil {
		t.Fatalf("GET state: %v", err)
	}
	var state struct {
		SetupRequired bool `json:"setup_required"`
	}
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !state.SetupRequired {
		t.Fatalf("state status/setup = %d/%t", response.StatusCode, state.SetupRequired)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/messages", strings.NewReader(`{"request_id":"no-provider","content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", baseURL)
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("POST without provider: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("POST without provider status = %d, want 503", response.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("web command shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("web command did not stop after context cancellation")
	}
	if got := output.String(); strings.Contains(got, "\x1b[") || strings.TrimSpace(got) != line {
		t.Fatalf("terminal output = %q, want only server status and no TUI", got)
	}
}

func TestWebPortConflictCreatesNoProfileDatabase(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", appDir)
	occupied, err := listenWeb(0)
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port
	cmd := webCmd()
	cmd.SetArgs([]string{"--port", strconv.Itoa(port)})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("web command succeeded on an occupied port")
	}
	dbPath, err := config.ProfileDBPath("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("profile database exists after bind failure: stat err=%v", err)
	}
}

func TestWebShutdownWaitsForSendBeforeDBClose(t *testing.T) {
	db, err := store.OpenForTesting(t.TempDir() + "/shutdown.db")
	if err != nil {
		t.Fatal(err)
	}
	session := &webShutdownSession{started: make(chan struct{}), release: make(chan struct{})}
	service := web.NewService(db, &store.Profile{ID: "p1", Name: "Test"}, session, "model", false)
	sendDone := make(chan error, 1)
	go func() {
		_, err := service.Send(context.Background(), "active", "hello")
		sendDone <- err
	}()
	select {
	case <-session.started:
	case <-time.After(2 * time.Second):
		t.Fatal("send did not start")
	}

	var mu sync.Mutex
	var closed []string
	cleanupDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cleanupDone <- cleanupWeb(ctx, nil, service, func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			closed = append(closed, "session")
			return nil
		}, func() error {
			mu.Lock()
			defer mu.Unlock()
			closed = append(closed, "db")
			return db.Close()
		})
	}()
	select {
	case err := <-cleanupDone:
		t.Fatalf("cleanup finished before pending send: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	mu.Lock()
	if len(closed) != 0 {
		t.Fatalf("resources closed while send active: %v", closed)
	}
	mu.Unlock()
	close(session.release)
	select {
	case err := <-cleanupDone:
		if err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup did not finish after send")
	}
	if err := <-sendDone; err != nil {
		t.Fatalf("send: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !bytes.Equal([]byte(strings.Join(closed, ",")), []byte("session,db")) {
		t.Fatalf("close order = %v, want [session db]", closed)
	}
}

type notifyWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
	ch  chan struct{}
}

func newNotifyWriter() *notifyWriter { return &notifyWriter{ch: make(chan struct{}, 1)} }
func (w *notifyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.buf.Write(p)
	w.mu.Unlock()
	select {
	case w.ch <- struct{}{}:
	default:
	}
	return n, err
}
func (w *notifyWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}
func (w *notifyWriter) waitFor(t *testing.T, text string) string {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		if output := w.String(); strings.Contains(output, text) {
			return strings.TrimSpace(output)
		}
		select {
		case <-w.ch:
		case <-timer.C:
			t.Fatalf("timed out waiting for %q; output=%q", text, w.String())
		}
	}
}

type webShutdownSession struct {
	started chan struct{}
	release chan struct{}
}

func (s *webShutdownSession) PendingNotes() int { return 0 }
func (s *webShutdownSession) Send(context.Context, string) (*chat.SendResult, error) {
	close(s.started)
	<-s.release
	return &chat.SendResult{Reply: "complete"}, nil
}
