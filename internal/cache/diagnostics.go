package cache

import (
	"maps"
	"time"
)

// Snapshot is an aggregated view of cache diagnostics.
type Snapshot struct {
	TotalRequests      int
	Hits               int
	Misses             int
	HitRate            float64
	MissRate           float64
	AverageRebuildTime time.Duration
	TopMissReasons     map[string]int
}

// Diagnostics accumulates request-level cache metrics.
type Diagnostics struct {
	total      int
	hits       int
	misses     int
	rebuildSum time.Duration
	rebuildCnt int
	reasons    map[string]int
}

// NewDiagnostics creates a diagnostics collector.
func NewDiagnostics() *Diagnostics {
	return &Diagnostics{reasons: map[string]int{}}
}

// RecordHit increments cache hit counters.
func (d *Diagnostics) RecordHit(_ time.Duration) {
	d.total++
	d.hits++
}

// RecordMiss increments miss counters and rebuild timing.
func (d *Diagnostics) RecordMiss(reason string, rebuild time.Duration) {
	d.total++
	d.misses++
	if reason != "" {
		d.reasons[reason]++
	}
	d.rebuildSum += rebuild
	d.rebuildCnt++
}

// Snapshot returns a copy of current diagnostics.
func (d *Diagnostics) Snapshot() Snapshot {
	s := Snapshot{TotalRequests: d.total, Hits: d.hits, Misses: d.misses, TopMissReasons: map[string]int{}}
	if d.total > 0 {
		s.HitRate = float64(d.hits) / float64(d.total)
		s.MissRate = float64(d.misses) / float64(d.total)
	}
	if d.rebuildCnt > 0 {
		s.AverageRebuildTime = d.rebuildSum / time.Duration(d.rebuildCnt)
	}
	maps.Copy(s.TopMissReasons, d.reasons)
	return s
}
