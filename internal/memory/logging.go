package memory

import "fmt"

// CaptureLogHook receives note capture lifecycle events.
type CaptureLogHook interface {
	CandidateScored(candidate NoteCandidate)
	DuplicateDetected(candidate NoteCandidate, existingID string)
	NoteStored(candidate NoteCandidate, noteID string)
	NoteStorageFailed(candidate NoteCandidate, err error)
	ExtractionPayloadRejected(reason string)
}

// NoopCaptureLogHook is a default hook that does nothing.
type NoopCaptureLogHook struct{}

// CandidateScored is a no-op event handler.
func (NoopCaptureLogHook) CandidateScored(NoteCandidate) {}

// DuplicateDetected is a no-op event handler.
func (NoopCaptureLogHook) DuplicateDetected(NoteCandidate, string) {}

// NoteStored is a no-op event handler.
func (NoopCaptureLogHook) NoteStored(NoteCandidate, string) {}

// NoteStorageFailed is a no-op event handler.
func (NoopCaptureLogHook) NoteStorageFailed(NoteCandidate, error) {}

// ExtractionPayloadRejected is a no-op event handler.
func (NoopCaptureLogHook) ExtractionPayloadRejected(string) {}

func rejectErr(msg string, args ...any) error {
	return fmt.Errorf(msg, args...)
}
