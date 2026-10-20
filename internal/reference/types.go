package reference

import "errors"

var (
	ErrUnsupported = errors.New("reference: unsupported file type")
	ErrUnsafePath  = errors.New("reference: unsafe file path")
	ErrInvalidText = errors.New("reference: text is not valid UTF-8")
	ErrNoText      = errors.New("reference: file contains no extractable text")
	ErrUnreadable  = errors.New("reference: file cannot be read")
	ErrTooLarge    = errors.New("reference: file exceeds size limit")
	ErrChanged     = errors.New("reference: file changed while being read")
)

type Document struct {
	Text, Fingerprint, MediaType string
	Size, ModTimeNS              int64
}
