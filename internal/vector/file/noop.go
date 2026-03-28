package file

import "io"

// NoopCodec provides a codec that returns empty payloads for benchmarking.
type NoopCodec struct{}

func NewNoopCodec() *NoopCodec {
	return &NoopCodec{}
}

func (c *NoopCodec) ReadHeader(_ io.Reader) (*Header, error) { return &Header{}, nil }
func (c *NoopCodec) WriteHeader(_ io.Writer, _ Header) error { return nil }
func (c *NoopCodec) ReadVectors(_ io.Reader, _ int, _ int) ([]float32, error) { return nil, nil }
func (c *NoopCodec) WriteVectors(_ io.Writer, _ []float32, _ int) error { return nil }
func (c *NoopCodec) ReadGraph(_ io.Reader) ([]byte, error) { return nil, nil }
func (c *NoopCodec) WriteGraph(_ io.Writer, _ []byte) error { return nil }
