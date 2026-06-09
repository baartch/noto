package integration

import (
	"testing"
	"time"

	"noto/internal/cache"
)

func TestMemoryDiagnostics_ExposeMissReasonsAndRollupActivityFields(t *testing.T) {
	d := cache.NewDiagnostics()
	d.RecordMiss("timeline_settings_changed", 5*time.Millisecond)
	d.RecordHit(2 * time.Millisecond)
	s := d.Snapshot()
	if s.TopMissReasons["timeline_settings_changed"] == 0 {
		t.Fatalf("expected miss reason to be tracked")
	}
	if s.Hits == 0 || s.Misses == 0 {
		t.Fatalf("expected hit/miss tracking in snapshot: %#v", s)
	}
}
