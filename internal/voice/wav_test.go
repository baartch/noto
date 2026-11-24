package voice

import (
	"encoding/binary"
	"errors"
	"testing"
)

func makePCM16WAV(channels uint16, sampleRate uint32, bitsPerSample uint16, samples []byte) []byte {
	padding := len(samples) % 2
	wav := make([]byte, 44+len(samples)+padding)
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], channels)
	binary.LittleEndian.PutUint32(wav[24:28], sampleRate)
	blockAlign := channels * bitsPerSample / 8
	binary.LittleEndian.PutUint32(wav[28:32], sampleRate*uint32(blockAlign))
	binary.LittleEndian.PutUint16(wav[32:34], blockAlign)
	binary.LittleEndian.PutUint16(wav[34:36], bitsPerSample)
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(len(samples)))
	copy(wav[44:], samples)
	return wav
}

func appendWAVChunk(wav []byte, id string, payload []byte, withPadding bool) []byte {
	out := append([]byte(nil), wav...)
	out = append(out, id...)
	chunkSize := make([]byte, 4)
	binary.LittleEndian.PutUint32(chunkSize, uint32(len(payload)))
	out = append(out, chunkSize...)
	out = append(out, payload...)
	if withPadding && len(payload)%2 != 0 {
		out = append(out, 0)
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}

func insertWAVChunkAfterFormat(wav []byte, id string, payload []byte, withPadding bool) []byte {
	out := append([]byte(nil), wav[:36]...)
	chunk := appendWAVChunk([]byte("RIFF\x00\x00\x00\x00WAVE"), id, payload, withPadding)
	out = append(out, chunk[12:]...)
	out = append(out, wav[36:]...)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}

func extendWAVFormat(wav []byte) []byte {
	out := make([]byte, len(wav)+2)
	copy(out[:36], wav[:36])
	copy(out[36:38], []byte{0, 0})
	copy(out[38:], wav[36:])
	binary.LittleEndian.PutUint32(out[16:20], 18)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}

func TestValidateWAV(t *testing.T) {
	valid := makePCM16WAV(1, 16000, 16, []byte{0x01, 0x02})
	maxWAV := makePCM16WAV(1, 16000, 16, make([]byte, 16000*300*2))
	oneByteOver := append(append([]byte(nil), maxWAV...), 0)
	badRIFFSize := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badRIFFSize[4:8], 1)
	badDataSize := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badDataSize[40:44], 1)
	dataChunkOverrun := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(dataChunkOverrun[40:44], 100)
	floatWAV := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint16(floatWAV[20:22], 3)
	listChunk := insertWAVChunkAfterFormat(valid, "LIST", []byte("INF"), true)
	listChunkMissingPad := insertWAVChunkAfterFormat(valid, "LIST", []byte("INF"), false)
	duplicateFormat := insertWAVChunkAfterFormat(valid, "fmt ", valid[20:36], true)
	duplicateData := appendWAVChunk(valid, "data", []byte{3, 4}, true)
	extendedFormat := extendWAVFormat(valid)

	for _, tc := range []struct {
		name    string
		data    []byte
		want    bool
		wantErr error
	}{
		{name: "mono PCM16 16 kHz", data: valid, want: true},
		{name: "exact five-minute size", data: maxWAV, want: true},
		{name: "one byte over size limit", data: oneByteOver, wantErr: ErrWAVTooLarge},
		{name: "truncated header", data: valid[:20]},
		{name: "inconsistent RIFF size", data: badRIFFSize},
		{name: "inconsistent data size", data: badDataSize},
		{name: "chunk overruns file", data: dataChunkOverrun},
		{name: "stereo", data: makePCM16WAV(2, 16000, 16, []byte{1, 2, 3, 4})},
		{name: "wrong sample rate", data: makePCM16WAV(1, 44100, 16, []byte{1, 2})},
		{name: "wrong bit depth", data: makePCM16WAV(1, 16000, 8, []byte{1, 2})},
		{name: "non-PCM encoding", data: floatWAV},
		{name: "empty data chunk", data: makePCM16WAV(1, 16000, 16, nil)},
		{name: "unaligned data chunk", data: makePCM16WAV(1, 16000, 16, []byte{1, 2, 3})},
		{name: "odd metadata chunk with padding", data: listChunk},
		{name: "odd metadata chunk without padding", data: listChunkMissingPad},
		{name: "duplicate format chunk", data: duplicateFormat},
		{name: "duplicate data chunk", data: duplicateData},
		{name: "extended format chunk", data: extendedFormat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWAV(tc.data)
			if tc.want && err != nil {
				t.Fatalf("ValidateWAV() error = %v", err)
			}
			if !tc.want && err == nil {
				t.Fatal("ValidateWAV() succeeded, want error")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateWAV() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
