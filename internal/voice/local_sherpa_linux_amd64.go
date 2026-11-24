//go:build linux && amd64 && cgo && !musl

package voice

/*
#cgo LDFLAGS: -Wl,-rpath,$ORIGIN/lib
*/
import "C"

import (
	"context"
	"errors"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

func localRuntimeAvailable() bool { return true }

func localRuntimeUnavailableReason() string { return "" }

func newSherpaRecognizer(paths modelPaths) (localRecognizer, error) {
	recognizer := sherpa.NewOfflineRecognizer(&sherpa.OfflineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{SampleRate: 16000, FeatureDim: 80},
		ModelConfig: sherpa.OfflineModelConfig{
			Transducer: sherpa.OfflineTransducerModelConfig{
				Encoder: paths.Encoder,
				Decoder: paths.Decoder,
				Joiner:  paths.Joiner,
			},
			Tokens:     paths.Tokens,
			NumThreads: 2,
			Provider:   "cpu",
			ModelType:  "nemo_transducer",
		},
		DecodingMethod: "greedy_search",
	})
	if recognizer == nil {
		return nil, errors.New("sherpa-onnx could not load the Parakeet model")
	}
	return &sherpaLocalRecognizer{recognizer: recognizer}, nil
}

type sherpaLocalRecognizer struct {
	recognizer *sherpa.OfflineRecognizer
}

func (r *sherpaLocalRecognizer) Transcribe(ctx context.Context, samples []float32) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(samples) == 0 {
		return "", ErrInvalidWAV
	}
	stream := sherpa.NewOfflineStream(r.recognizer)
	if stream == nil {
		return "", errors.New("sherpa-onnx could not create a recognition stream")
	}
	defer sherpa.DeleteOfflineStream(stream)
	stream.AcceptWaveform(16000, samples)
	r.recognizer.Decode(stream)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	result := stream.GetResult()
	if result == nil {
		return "", errors.New("sherpa-onnx returned no recognition result")
	}
	return result.Text, nil
}

func (r *sherpaLocalRecognizer) Close() error {
	if r.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(r.recognizer)
		r.recognizer = nil
	}
	return nil
}
