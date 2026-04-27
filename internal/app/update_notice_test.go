package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestStartUpdateCheckAsync_NonBlocking(t *testing.T) {
	start := time.Now()
	startUpdateCheckAsync(func(tea.Msg) {})
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("startUpdateCheckAsync appears blocking")
	}
}
