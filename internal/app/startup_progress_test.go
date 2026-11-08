package app

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"noto/internal/startup"
)

type notifyingWriter struct {
	mu     sync.Mutex
	b      bytes.Buffer
	writes chan struct{}
}

func (w *notifyingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.b.Write(p)
	w.mu.Unlock()
	select {
	case w.writes <- struct{}{}:
	default:
	}
	return n, err
}

func TestStartupDisplayPlainOutputShowsProgressAndWarning(t *testing.T) {
	var out bytes.Buffer
	display := newStartupDisplay(&out, false)
	display.Report(startup.Event{Phase: "files", State: "start", Total: -1})
	display.Report(startup.Event{Phase: "files", State: "update", Done: 1, Total: 2})
	display.Report(startup.Event{Phase: "files", State: "warning", Done: 2, Total: 2})
	display.Close()
	got := out.String()
	for _, text := range []string{"files", "1/2", "warning"} {
		if !strings.Contains(got, text) {
			t.Errorf("output %q missing %q", got, text)
		}
	}
	if strings.ContainsAny(got, "\r\x1b") {
		t.Errorf("redirected output contains terminal controls: %q", got)
	}
}

func TestWithStartupProgressReportsFailureAndStops(t *testing.T) {
	var out bytes.Buffer
	var report startup.Reporter
	want := errors.New("private detail")
	err := withStartupProgress(&out, func(r startup.Reporter) error {
		report = r
		r(startup.Event{Phase: "sync", State: "start", Total: 2})
		return want
	})
	if !errors.Is(err, want) || !strings.Contains(out.String(), "failed") || strings.Contains(out.String(), "private detail") {
		t.Fatalf("error = %v, output = %q", err, out.String())
	}
	before := out.Len()
	report(startup.Event{Phase: "sync", State: "success", Done: 2, Total: 2})
	if out.Len() != before {
		t.Fatal("startup display wrote after session initialization")
	}
}

func TestStartupDisplayTerminalBarAndFailure(t *testing.T) {
	var out bytes.Buffer
	display := newStartupDisplay(&out, true)
	display.Report(startup.Event{Phase: "rebuild", State: "start", Total: 2})
	display.Report(startup.Event{Phase: "rebuild", State: "update", Done: 1, Total: 2})
	display.Report(startup.Event{Phase: "rebuild", State: "failure", Done: 1, Total: 2})
	display.Close()
	got := out.String()
	for _, text := range []string{"[", "1/2", "failed"} {
		if !strings.Contains(got, text) {
			t.Errorf("output %q missing %q", got, text)
		}
	}
	before := out.Len()
	display.Report(startup.Event{Phase: "rebuild", State: "success", Done: 2, Total: 2})
	if out.Len() != before {
		t.Fatal("output changed after Close")
	}
}

func TestStartupDisplayAnimatesWhileWaiting(t *testing.T) {
	out := &notifyingWriter{writes: make(chan struct{}, 10)}
	display := newStartupDisplay(out, true)
	display.Report(startup.Event{Phase: "rebuild", State: "start", Total: 1})
	<-out.writes
	select {
	case <-out.writes:
	case <-time.After(time.Second):
		display.Close()
		t.Fatal("spinner did not update while waiting")
	}
	display.Close()
}
