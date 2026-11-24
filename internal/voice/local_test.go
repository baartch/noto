package voice

import (
	"math"
	"testing"
)

func TestPCM16WAVSamples(t *testing.T) {
	wav := makePCM16WAV(1, 16000, 16, []byte{0x00, 0x80, 0xff, 0x7f})
	samples, err := pcm16WAVSamples(wav)
	if err != nil {
		t.Fatalf("pcm16WAVSamples(): %v", err)
	}
	if len(samples) != 2 || samples[0] != -1 || math.Abs(float64(samples[1]-32767.0/32768.0)) > 1e-6 {
		t.Fatalf("samples = %#v, want [-1, 32767/32768]", samples)
	}
}

func TestParakeetV3ManifestPinsHuggingFaceFiles(t *testing.T) {
	want := map[string]modelFileSpec{
		"encoder.int8.onnx": {Name: "encoder.int8.onnx", Size: 652_282_294, SHA256: "1afbe69c1ffdabee94755f42e4daf1d93549131a3da11356bda84fcef0f2c479"},
		"decoder.int8.onnx": {Name: "decoder.int8.onnx", Size: 11_845_274, SHA256: "0222a86147299d4dee90edf95b6d13827edb42ad8b7c9cf0b2582f030937c0b5"},
		"joiner.int8.onnx":  {Name: "joiner.int8.onnx", Size: 6_355_277, SHA256: "cbd02f3696e9944970111fb439d9c3fd365a80374565f13f2b9c7007d253fc98"},
		"tokens.txt":        {Name: "tokens.txt", Size: 93_939, SHA256: "d58544679ea4bc6ac563d1f545eb7d474bd6cfa467f0a6e2c1dc1c7d37e3c35d"},
	}
	got := parakeetV3Manifest()
	if len(got) != len(want) {
		t.Fatalf("manifest contains %d files, want %d", len(got), len(want))
	}
	for _, file := range got {
		if expected, ok := want[file.Name]; !ok || expected != file {
			t.Errorf("manifest file %#v does not match pinned HF metadata", file)
		}
	}
	if size := manifestSize(got); size != 670_576_784 {
		t.Fatalf("model file size = %d, want 670576784", size)
	}
}
