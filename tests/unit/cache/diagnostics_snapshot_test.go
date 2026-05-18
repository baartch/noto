package cache_test

import (
	"testing"
	"time"

	"noto/internal/cache"
)

func TestDiagnosticsSnapshot_AverageRebuildTime(t *testing.T) {
	d := cache.NewDiagnostics()
	d.RecordMiss("not_found", 10*time.Millisecond)
	d.RecordMiss("not_found", 30*time.Millisecond)
	s := d.Snapshot()
	if s.AverageRebuildTime != 20*time.Millisecond {
		t.Fatalf("got %s", s.AverageRebuildTime)
	}
}
