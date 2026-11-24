package voice

import "context"

// Transcriber converts a complete audio recording into text.
type Transcriber interface {
	Transcribe(context.Context, []byte) (string, error)
}
