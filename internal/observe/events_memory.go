package observe

import "time"

// EmitRetrieval emits a retrieval observability event.
func EmitRetrieval(l Logger, profileID string, cacheHit bool, latency time.Duration) {
	status := StatusSuccess
	l.Emit(Event{
		EventType: EventRetrieval,
		ProfileID: profileID,
		Status:    status,
		LatencyMs: LatencyPtr(latency),
		Metadata:  map[string]any{"cache_hit": cacheHit},
	})
}

// EmitCacheEvent emits a cache hit or miss event.
func EmitCacheEvent(l Logger, profileID string, hit bool, latency time.Duration) {
	l.Emit(Event{
		EventType: EventCache,
		ProfileID: profileID,
		Status:    StatusSuccess,
		LatencyMs: LatencyPtr(latency),
		Metadata:  map[string]any{"hit": hit},
	})
}

// EmitSlashParse emits a slash parse event.
func EmitSlashParse(l Logger, profileID, commandPath string, ok bool) {
	status := StatusSuccess
	if !ok {
		status = StatusFailure
	}
	l.Emit(Event{
		EventType: EventSlashParse,
		ProfileID: profileID,
		Status:    status,
		Metadata:  map[string]any{"command_path": commandPath},
	})
}

// EmitSlashSuggest emits a slash suggestion event.
func EmitSlashSuggest(l Logger, profileID, prefix string, count int) {
	l.Emit(Event{
		EventType: EventSlashSuggest,
		ProfileID: profileID,
		Status:    StatusSuccess,
		Metadata:  map[string]any{"prefix": prefix, "count": count},
	})
}

// EmitSlashExecute emits a slash command execution event.
func EmitSlashExecute(l Logger, profileID, commandPath string, latency time.Duration, err error) {
	status := StatusSuccess
	meta := map[string]any{"command_path": commandPath}
	if err != nil {
		status = StatusFailure
		meta["error"] = err.Error()
	}
	l.Emit(Event{
		EventType: EventSlashExecute,
		ProfileID: profileID,
		Status:    status,
		LatencyMs: LatencyPtr(latency),
		Metadata:  meta,
	})
}
