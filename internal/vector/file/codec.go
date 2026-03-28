package file

import (
	"encoding/binary"
	"fmt"
	"io"
)

const Magic = "NOTOVEC1"

// Header describes the on-disk vector file header.
type Header struct {
	ProfileID      string
	EmbeddingModel string
	EmbeddingDim   uint32
	EntryCount     uint32
	VectorsOffset  uint64
	GraphOffset    uint64
}

// Codec defines operations for reading/writing the vector index file.
type Codec interface {
	ReadHeader(r io.Reader) (*Header, error)
	WriteHeader(w io.Writer, h Header) error
	ReadVectors(r io.Reader, count int, dim int) ([]float32, error)
	WriteVectors(w io.Writer, vectors []float32, dim int) error
	ReadGraph(r io.Reader) ([]byte, error)
	WriteGraph(w io.Writer, data []byte) error
}

// BinaryCodec reads/writes the vector index file format.
type BinaryCodec struct{}

// NewBinaryCodec returns a BinaryCodec.
func NewBinaryCodec() *BinaryCodec {
	return &BinaryCodec{}
}

func (c *BinaryCodec) ReadHeader(r io.Reader) (*Header, error) {
	magic := make([]byte, len(Magic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("vector: read magic: %w", err)
	}
	if string(magic) != Magic {
		return nil, fmt.Errorf("vector: invalid magic")
	}

	profileID, err := readString(r)
	if err != nil {
		return nil, err
	}
	model, err := readString(r)
	if err != nil {
		return nil, err
	}
	var header Header
	if err := binary.Read(r, binary.LittleEndian, &header.EmbeddingDim); err != nil {
		return nil, fmt.Errorf("vector: read dim: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.EntryCount); err != nil {
		return nil, fmt.Errorf("vector: read count: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.VectorsOffset); err != nil {
		return nil, fmt.Errorf("vector: read vectors offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &header.GraphOffset); err != nil {
		return nil, fmt.Errorf("vector: read graph offset: %w", err)
	}
	header.ProfileID = profileID
	header.EmbeddingModel = model
	return &header, nil
}

func (c *BinaryCodec) WriteHeader(w io.Writer, h Header) error {
	if _, err := w.Write([]byte(Magic)); err != nil {
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
	if err := binary.Write(w, binary.LittleEndian, h.GraphOffset); err != nil {
		return fmt.Errorf("vector: write graph offset: %w", err)
	}
	return nil
}

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

func (c *BinaryCodec) WriteVectors(w io.Writer, vectors []float32, _ int) error {
	if len(vectors) == 0 {
		return nil
	}
	if err := binary.Write(w, binary.LittleEndian, vectors); err != nil {
		return fmt.Errorf("vector: write vectors: %w", err)
	}
	return nil
}

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
