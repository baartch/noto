package reference

import "errors"

var (
	// ErrUnsupported identifies an unsupported file extension.
	ErrUnsupported = errors.New("reference: unsupported file type")
	// ErrUnsafePath identifies a path that escapes the rooted profile directory or uses a symlink.
	ErrUnsafePath = errors.New("reference: unsafe file path")
	// ErrInvalidText identifies non-UTF-8 TXT or MD content.
	ErrInvalidText = errors.New("reference: text is not valid UTF-8")
	// ErrNoText identifies empty or image-only source content.
	ErrNoText = errors.New("reference: file contains no extractable text")
	// ErrUnreadable identifies malformed, encrypted, or inaccessible source content.
	ErrUnreadable = errors.New("reference: file cannot be read")
	// ErrTooLarge identifies a source or extracted text that exceeds configured limits.
	ErrTooLarge = errors.New("reference: file exceeds size limit")
	// ErrChanged identifies a source modified during a read or indexing operation.
	ErrChanged = errors.New("reference: file changed while being read")
)

// Document is one bounded source snapshot and its extracted text.
type Document struct {
	Text, Fingerprint, MediaType string
	Size, ModTimeNS              int64
}
