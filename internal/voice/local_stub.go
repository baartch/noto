//go:build !linux || !amd64 || !cgo || musl

package voice

func localRuntimeAvailable() bool { return false }

func localRuntimeUnavailableReason() string {
	return "Local Parakeet requires a Linux amd64 glibc build with cgo enabled."
}

func newSherpaRecognizer(modelPaths) (localRecognizer, error) {
	return nil, ErrLocalRuntimeUnavailable
}
