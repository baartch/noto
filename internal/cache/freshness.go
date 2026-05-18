package cache

import "time"

// FreshnessState classifies cache entry staleness.
type FreshnessState string

const (
	// Fresh indicates entry is not expired.
	Fresh FreshnessState = "fresh"
	// SlightlyStale indicates entry is expired but still in SWR window.
	SlightlyStale FreshnessState = "slightly_stale"
	// Stale indicates entry is too old and should not be served.
	Stale FreshnessState = "stale"
)

// StateForExpiry computes freshness based on expiry and SWR window.
func StateForExpiry(expiresAt *time.Time, now time.Time, swrWindow time.Duration) FreshnessState {
	if expiresAt == nil || expiresAt.After(now) {
		return Fresh
	}
	if expiresAt.After(now.Add(-swrWindow)) {
		return SlightlyStale
	}
	return Stale
}
