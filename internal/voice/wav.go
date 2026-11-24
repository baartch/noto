package voice

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const maxTranscriptionWAVBytes = 9_600_044

var (
	// ErrInvalidWAV indicates the audio is not canonical mono 16 kHz PCM16 WAV.
	ErrInvalidWAV = errors.New("voice: invalid WAV audio")
	// ErrWAVTooLarge indicates the recording exceeds the five-minute limit.
	ErrWAVTooLarge = errors.New("voice: WAV audio exceeds five minutes")
)

// ValidateWAV accepts canonical 44-byte-header mono, 16 kHz PCM16 RIFF/WAVE
// recordings up to five minutes long. Extra chunks and metadata are rejected.
func ValidateWAV(data []byte) error {
	if len(data) > maxTranscriptionWAVBytes {
		return ErrWAVTooLarge
	}
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return fmt.Errorf("%w: missing canonical RIFF/WAVE header", ErrInvalidWAV)
	}
	if uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return fmt.Errorf("%w: inconsistent RIFF size", ErrInvalidWAV)
	}
	if string(data[12:16]) != "fmt " || binary.LittleEndian.Uint32(data[16:20]) != 16 {
		return fmt.Errorf("%w: expected 16-byte format chunk", ErrInvalidWAV)
	}
	if string(data[36:40]) != "data" {
		return fmt.Errorf("%w: expected data chunk after format", ErrInvalidWAV)
	}
	dataBytes := uint64(binary.LittleEndian.Uint32(data[40:44]))
	if dataBytes+44 != uint64(len(data)) {
		return fmt.Errorf("%w: inconsistent data size or unexpected metadata", ErrInvalidWAV)
	}
	format := binary.LittleEndian.Uint16(data[20:22])
	channels := binary.LittleEndian.Uint16(data[22:24])
	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	byteRate := binary.LittleEndian.Uint32(data[28:32])
	blockAlign := binary.LittleEndian.Uint16(data[32:34])
	bitsPerSample := binary.LittleEndian.Uint16(data[34:36])
	if format != 1 || channels != 1 || sampleRate != 16000 || byteRate != 32000 || blockAlign != 2 || bitsPerSample != 16 {
		return fmt.Errorf("%w: expected mono 16 kHz PCM16", ErrInvalidWAV)
	}
	if dataBytes == 0 || dataBytes%uint64(blockAlign) != 0 {
		return fmt.Errorf("%w: empty or incomplete PCM frame", ErrInvalidWAV)
	}
	return nil
}
