package memory

// CaptureLogHook receives note capture lifecycle events.
type CaptureLogHook interface {
	CandidateScored(candidate NoteCandidate)
	DuplicateDetected(candidate NoteCandidate, existingID string)
	NoteStored(candidate NoteCandidate, noteID string)
	NoteStorageFailed(candidate NoteCandidate, err error)
}

// NoopCaptureLogHook is a default hook that does nothing.
type NoopCaptureLogHook struct{}

func (NoopCaptureLogHook) CandidateScored(NoteCandidate)                     {}
func (NoopCaptureLogHook) DuplicateDetected(NoteCandidate, string)          {}
func (NoopCaptureLogHook) NoteStored(NoteCandidate, string)                 {}
func (NoopCaptureLogHook) NoteStorageFailed(NoteCandidate, error)           {}
