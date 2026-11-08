package app

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/term"

	"noto/internal/startup"
)

type startupDisplay struct {
	mu       sync.Mutex
	writer   io.Writer
	terminal bool
	event    startup.Event
	active   bool
	closed   bool
	frame    int
	lastDone int
	stop     chan struct{}
	done     chan struct{}
}

func newStartupDisplay(w io.Writer, terminal bool) *startupDisplay {
	d := &startupDisplay{writer: w, terminal: terminal, lastDone: -1}
	if terminal {
		d.stop, d.done = make(chan struct{}), make(chan struct{})
		go d.animate()
	}
	return d
}

func (d *startupDisplay) animate() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	defer close(d.done)
	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
			d.mu.Lock()
			if d.active && !d.closed {
				d.frame++
				d.render(false)
			}
			d.mu.Unlock()
		}
	}
}

func (d *startupDisplay) Report(event startup.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if event.Phase != d.event.Phase || event.State == "start" {
		d.lastDone = -1
	}
	d.event = event
	d.active = event.State == "start" || event.State == "update"
	if d.terminal {
		d.render(!d.active)
		return
	}
	// Avoid flooding redirected output for large profiles.
	step := max(1, event.Total/10)
	if event.State != "update" || d.lastDone < 0 || event.Done-d.lastDone >= step || event.Done == event.Total {
		d.render(true)
		d.lastDone = event.Done
	}
}

// render is called with mu held.
func (d *startupDisplay) render(final bool) {
	e := d.event
	label := map[string]string{
		"starting": "Starting Noto", "files": "Scanning profile files",
		"checking": "Checking vector index", "rebuild": "Rebuilding vector index", "sync": "Adding missing notes to vector index",
	}[e.Phase]
	if label == "" {
		label = "Starting Noto"
	}
	line := label
	if e.Total >= 0 {
		if d.terminal && e.Total > 0 {
			filled := min(20, max(0, e.Done*20/e.Total))
			line += " [" + strings.Repeat("=", filled) + strings.Repeat("-", 20-filled) + "]"
		}
		line += fmt.Sprintf(" %d/%d", e.Done, e.Total)
	}
	switch e.State {
	case "success":
		line += " - done"
	case "warning":
		line += " - warning: some files could not be processed"
	case "failure":
		line += " - failed; see log for details"
	default:
		if d.terminal {
			line += " " + string("|/-\\"[d.frame%4])
		}
		if e.Total > 0 && e.Done == e.Total {
			line += " finalizing"
		}
	}
	if d.terminal {
		_, _ = fmt.Fprintf(d.writer, "\r\x1b[2K%s", line)
		if final {
			_, _ = fmt.Fprintln(d.writer)
		}
	} else {
		_, _ = fmt.Fprintln(d.writer, line)
	}
}

func (d *startupDisplay) Close() {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	if d.terminal && d.active {
		_, _ = fmt.Fprint(d.writer, "\r\x1b[2K")
	}
	d.mu.Unlock()
	if d.terminal {
		close(d.stop)
		<-d.done
	}
}

func withStartupProgress(w io.Writer, run func(startup.Reporter) error) error {
	terminal := false
	if file, ok := w.(interface{ Fd() uintptr }); ok {
		terminal = term.IsTerminal(file.Fd())
	}
	display := newStartupDisplay(w, terminal)
	defer display.Close()
	display.Report(startup.Event{Phase: "starting", State: "start", Total: -1})
	err := run(display.Report)
	state := "success"
	if err != nil {
		state = "failure"
	}
	display.Report(startup.Event{Phase: "starting", State: state, Total: -1})
	return err
}
