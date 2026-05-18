package cache_test

import (
	"testing"
	"time"

	"noto/internal/cache"
)

func TestDiagnosticsSnapshot_RatesAndReasons(t *testing.T) {
	d := cache.NewDiagnostics()
	d.RecordHit(1 * time.Millisecond)
	d.RecordMiss("prompt_changed", 20*time.Millisecond)
	s := d.Snapshot()
	if s.TotalRequests != 2 || s.Hits != 1 || s.Misses != 1 {
		t.Fatalf("unexpected counters: %+v", s)
	}
	if s.TopMissReasons["prompt_changed"] != 1 {
		t.Fatalf("expected miss reason count")
	}
}
