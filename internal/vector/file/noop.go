package file

import "io"

// NoopCodec provides a codec that returns empty payloads for benchmarking.
type NoopCodec struct{}

// NewNoopCodec creates a new NoopCodec.
func NewNoopCodec() *NoopCodec {
	return &NoopCodec{}
}

// ReadHeader returns an empty header.
func (c *NoopCodec) ReadHeader(_ io.Reader) (*Header, error) { return &Header{}, nil }

// WriteHeader is a no-op.
func (c *NoopCodec) WriteHeader(_ io.Writer, _ Header) error { return nil }

// ReadVectors returns no vectors.
func (c *NoopCodec) ReadVectors(_ io.Reader, _ int, _ int) ([]float32, error) { return nil, nil }

// WriteVectors is a no-op.
func (c *NoopCodec) WriteVectors(_ io.Writer, _ []float32, _ int) error { return nil }

// ReadGraph returns an empty graph payload.
func (c *NoopCodec) ReadGraph(_ io.Reader) ([]byte, error) { return nil, nil }

// WriteGraph is a no-op.
func (c *NoopCodec) WriteGraph(_ io.Writer, _ []byte) error { return nil }
