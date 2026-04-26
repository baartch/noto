package app

import (
	"bytes"
	"testing"
	"time"
)

func TestStartUpdateCheckAsync_NonBlocking(t *testing.T) {
	buf := new(bytes.Buffer)
	start := time.Now()
	startUpdateCheckAsync(buf)
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("startUpdateCheckAsync appears blocking")
	}
}
