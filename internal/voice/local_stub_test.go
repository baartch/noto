//go:build !linux || !amd64 || !cgo || musl

package voice

import (
	"errors"
	"testing"
)

func TestLocalRuntimeStubReportsUnavailable(t *testing.T) {
	if localRuntimeAvailable() {
		t.Fatal("unsupported build reported local speech available")
	}
	if reason := localRuntimeUnavailableReason(); reason == "" {
		t.Fatal("unsupported build returned no explanation")
	}
	if _, err := newSherpaRecognizer(modelPaths{}); !errors.Is(err, ErrLocalRuntimeUnavailable) {
		t.Fatalf("newSherpaRecognizer() error = %v, want ErrLocalRuntimeUnavailable", err)
	}
}
