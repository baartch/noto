package observe

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// EventType categorises an observability event.
type EventType string

const (
	EventStartup      EventType = "startup"
	EventRetrieval    EventType = "retrieval"
	EventCache        EventType = "cache"
	EventProviderCall EventType = "provider_call"
	EventRecovery     EventType = "recovery"
	EventSlashParse   EventType = "slash_parse"
	EventSlashSuggest EventType = "slash_suggest"
	EventSlashExecute EventType = "slash_execute"
)

// Status represents the outcome of an event.
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
)

// Event is a structured observability event.
type Event struct {
	EventType EventType      `json:"event_type"`
	ProfileID string         `json:"profile_id,omitempty"`
	Status    Status         `json:"status"`
	LatencyMs *int64         `json:"latency_ms,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Logger is the structured event logger interface.
type Logger interface {
	// Emit records a structured event.
	Emit(e Event)
	// Infof logs a free-form informational message.
	Infof(format string, args ...any)
	// Errorf logs a free-form error message.
	Errorf(format string, args ...any)
}

// MetricsEmitter emits simple counter / gauge metrics.
type MetricsEmitter interface {
	// Inc increments a named counter by 1.
	Inc(name string)
	// RecordLatency records a latency observation in milliseconds.
	RecordLatency(name string, ms int64)
}

// ---- default implementations ------------------------------------------------

// JSONLogger writes JSON-encoded events to w.
type JSONLogger struct {
	mu sync.Mutex
	w  io.Writer
}

// NewJSONLogger creates a JSONLogger that writes to w.
// Pass os.Stderr for development; pass io.Discard or a file for production.
func NewJSONLogger(w io.Writer) *JSONLogger {
	return &JSONLogger{w: w}
}

// NewNoopLogger returns a Logger that silently discards all events.
func NewNoopLogger() Logger {
	return NewJSONLogger(io.Discard)
}

// NewStderrLogger returns a JSONLogger writing to stderr.
func NewStderrLogger() Logger {
	return NewJSONLogger(os.Stderr)
}

func (l *JSONLogger) Emit(e Event) {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	data, _ := json.Marshal(e)
	_, _ = fmt.Fprintf(l.w, "%s\n", data)
}

func (l *JSONLogger) Infof(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.w, "[INFO] "+format+"\n", args...)
}

func (l *JSONLogger) Errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.w, "[ERROR] "+format+"\n", args...)
}

// ---- noop metrics -----------------------------------------------------------

// NoopMetrics discards all metric observations.
type NoopMetrics struct{}

func (NoopMetrics) Inc(_ string)                    {}
func (NoopMetrics) RecordLatency(_ string, _ int64) {}

// ---- convenience helpers ----------------------------------------------------

// LatencyPtr converts a duration to a *int64 milliseconds pointer for use in Event.
func LatencyPtr(d time.Duration) *int64 {
	ms := d.Milliseconds()
	return &ms
}
