//go:build linux && amd64 && cgo && !musl

package voice

import (
	"testing"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

func TestSherpaRuntimeLoadsWithoutModelFiles(t *testing.T) {
	if !localRuntimeAvailable() {
		t.Fatal("Linux amd64 cgo build should support local speech")
	}
	if version := sherpa.GetVersion(); version == "" {
		t.Fatal("sherpa-onnx runtime returned an empty version")
	}
}
