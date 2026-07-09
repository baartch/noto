package file

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Magic identifies the vector file format version.
const (
	MagicV1 = "NOTOVEC1"
	MagicV2 = "NOTOVEC2"
)

// EntryRecord is the serialized metadata for one vector entry.
type EntryRecord struct {
	SourceType string
	SourceID   string
	ChunkHash  string
	VectorRef  string
}

// Header describes the on-disk vector file header.
type Header struct {
	ProfileID      string
	EmbeddingModel string
	EmbeddingDim   uint32
	EntryCount     uint32
	VectorsOffset  uint64
	EntriesOffset  uint64
	GraphOffset    uint64
	HasEntries     bool // true for V2+ format that includes entry metadata
}

// Codec defines operations for reading/writing the vector index file.
type Codec interface {
	ReadHeader(r io.Reader) (*Header, error)
	WriteHeader(w io.Writer, h Header) error
	ReadVectors(r io.Reader, count int, dim int) ([]float32, error)
	WriteVectors(w io.Writer, vectors []float32, dim int) error
	ReadEntries(r io.Reader, count int) ([]EntryRecord, error)
	WriteEntries(w io.Writer, entries []EntryRecord) error
	ReadGraph(r io.Reader) ([]byte, error)
	WriteGraph(w io.Writer, data []byte) error
}

// BinaryCodec reads/writes the vector index file format.
type BinaryCodec struct{}

// NewBinaryCodec returns a BinaryCodec.
func NewBinaryCodec() *BinaryCodec {
	return &BinaryCodec{}
}

// ReadHeader reads the vector file header. Supports V1 (NOTOVEC1) and V2 (NOTOVEC2) formats.
func (c *BinaryCodec) ReadHeader(r io.Reader) (*Header, error) {
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("vector: read magic: %w", err)
	}

	switch string(magic) {
	case MagicV2:
		return readV2Header(r)
	case MagicV1:
		return readV1Header(r)
	default:
		return nil, errors.New("vector: invalid magic")
	}
}

func readV1Header(r io.Reader) (*Header, error) {
	var h Header
	var err error
	h.ProfileID, err = readString(r)
	if err != nil {
		return nil, err
	}
	h.EmbeddingModel, err = readString(r)
	if err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &h.EmbeddingDim); err != nil {
		return nil, fmt.Errorf("vector: read dim: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.EntryCount); err != nil {
		return nil, fmt.Errorf("vector: read count: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.VectorsOffset); err != nil {
		return nil, fmt.Errorf("vector: read vectors offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.GraphOffset); err != nil {
		return nil, fmt.Errorf("vector: read graph offset: %w", err)
	}
	h.HasEntries = false
	return &h, nil
}

func readV2Header(r io.Reader) (*Header, error) {
	var h Header
	var err error
	h.ProfileID, err = readString(r)
	if err != nil {
		return nil, err
	}
	h.EmbeddingModel, err = readString(r)
	if err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &h.EmbeddingDim); err != nil {
		return nil, fmt.Errorf("vector: read dim: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.EntryCount); err != nil {
		return nil, fmt.Errorf("vector: read count: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.VectorsOffset); err != nil {
		return nil, fmt.Errorf("vector: read vectors offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.EntriesOffset); err != nil {
		return nil, fmt.Errorf("vector: read entries offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.GraphOffset); err != nil {
		return nil, fmt.Errorf("vector: read graph offset: %w", err)
	}
	h.HasEntries = true
	return &h, nil
}

// WriteHeader writes the vector file header in V2 format.
func (c *BinaryCodec) WriteHeader(w io.Writer, h Header) error {
	if _, err := w.Write([]byte(MagicV2)); err != nil {
		return fmt.Errorf("vector: write magic: %w", err)
	}
	if err := writeString(w, h.ProfileID); err != nil {
		return err
	}
	if err := writeString(w, h.EmbeddingModel); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.EmbeddingDim); err != nil {
		return fmt.Errorf("vector: write dim: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, h.EntryCount); err != nil {
		return fmt.Errorf("vector: write count: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, h.VectorsOffset); err != nil {
		return fmt.Errorf("vector: write vectors offset: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, h.EntriesOffset); err != nil {
		return fmt.Errorf("vector: write entries offset: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, h.GraphOffset); err != nil {
		return fmt.Errorf("vector: write graph offset: %w", err)
	}
	return nil
}

// ReadVectors reads vector data from the file.
func (c *BinaryCodec) ReadVectors(r io.Reader, count int, dim int) ([]float32, error) {
	if count == 0 || dim == 0 {
		return nil, nil
	}
	total := count * dim
	data := make([]float32, total)
	if err := binary.Read(r, binary.LittleEndian, data); err != nil {
		return nil, fmt.Errorf("vector: read vectors: %w", err)
	}
	return data, nil
}

// WriteVectors writes vector data to the file.
func (c *BinaryCodec) WriteVectors(w io.Writer, vectors []float32, _ int) error {
	if len(vectors) == 0 {
		return nil
	}
	if err := binary.Write(w, binary.LittleEndian, vectors); err != nil {
		return fmt.Errorf("vector: write vectors: %w", err)
	}
	return nil
}

// ReadGraph reads the serialized graph bytes.
func (c *BinaryCodec) ReadGraph(r io.Reader) ([]byte, error) {
	var size uint64
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("vector: read graph size: %w", err)
	}
	if size == 0 {
		return nil, nil
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("vector: read graph: %w", err)
	}
	return buf, nil
}

// WriteGraph writes the serialized graph bytes.
func (c *BinaryCodec) WriteGraph(w io.Writer, data []byte) error {
	size := uint64(len(data))
	if err := binary.Write(w, binary.LittleEndian, size); err != nil {
		return fmt.Errorf("vector: write graph size: %w", err)
	}
	if size == 0 {
		return nil
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("vector: write graph: %w", err)
	}
	return nil
}

// ReadEntries reads entry metadata from the file.
func (c *BinaryCodec) ReadEntries(r io.Reader, count int) ([]EntryRecord, error) {
	if count <= 0 {
		return nil, nil
	}
	entries := make([]EntryRecord, 0, count)
	for range count {
		var e EntryRecord
		var err error
		e.SourceType, err = readString(r)
		if err != nil {
			return nil, fmt.Errorf("vector: read entry source_type: %w", err)
		}
		e.SourceID, err = readString(r)
		if err != nil {
			return nil, fmt.Errorf("vector: read entry source_id: %w", err)
		}
		e.ChunkHash, err = readString(r)
		if err != nil {
			return nil, fmt.Errorf("vector: read entry chunk_hash: %w", err)
		}
		e.VectorRef, err = readString(r)
		if err != nil {
			return nil, fmt.Errorf("vector: read entry vector_ref: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// WriteEntries writes entry metadata to the file.
func (c *BinaryCodec) WriteEntries(w io.Writer, entries []EntryRecord) error {
	for _, e := range entries {
		if err := writeString(w, e.SourceType); err != nil {
			return fmt.Errorf("vector: write entry source_type: %w", err)
		}
		if err := writeString(w, e.SourceID); err != nil {
			return fmt.Errorf("vector: write entry source_id: %w", err)
		}
		if err := writeString(w, e.ChunkHash); err != nil {
			return fmt.Errorf("vector: write entry chunk_hash: %w", err)
		}
		if err := writeString(w, e.VectorRef); err != nil {
			return fmt.Errorf("vector: write entry vector_ref: %w", err)
		}
	}
	return nil
}

func readString(r io.Reader) (string, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return "", fmt.Errorf("vector: read string size: %w", err)
	}
	if size == 0 {
		return "", nil
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("vector: read string: %w", err)
	}
	return string(buf), nil
}

func writeString(w io.Writer, value string) error {
	data := []byte(value)
	size := uint32(len(data))
	if err := binary.Write(w, binary.LittleEndian, size); err != nil {
		return fmt.Errorf("vector: write string size: %w", err)
	}
	if size == 0 {
		return nil
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("vector: write string: %w", err)
	}
	return nil
}
