package memory

import (
	"fmt"
	"log"
)

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

// StdCaptureLogHook logs extractor/capture events via standard logger.
type StdCaptureLogHook struct{}

func (StdCaptureLogHook) CandidateScored(candidate NoteCandidate) {
	log.Printf("memory: candidate scored content=%q total=%d", candidate.Content, candidate.ValueScore.Total)
}

func (StdCaptureLogHook) DuplicateDetected(candidate NoteCandidate, existingID string) {
	log.Printf("memory: duplicate detected content=%q existing_id=%s", candidate.Content, existingID)
}

func (StdCaptureLogHook) NoteStored(candidate NoteCandidate, noteID string) {
	log.Printf("memory: note stored id=%s content=%q", noteID, candidate.Content)
}

func (StdCaptureLogHook) NoteStorageFailed(candidate NoteCandidate, err error) {
	log.Printf("memory: note store failed content=%q err=%v", candidate.Content, err)
}

func (StdCaptureLogHook) ExtractionPayloadRejected(reason string) {
	log.Printf("memory: extractor payload rejected: %s", reason)
}

func rejectErr(msg string, args ...any) error {
	return fmt.Errorf(msg, args...)
}
