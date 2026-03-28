package hnsw

// NoopGraph is a minimal graph implementation for tests.
type NoopGraph struct{}

func NewNoopGraph() *NoopGraph {
	return &NoopGraph{}
}

func (g *NoopGraph) Insert(_ string, _ []float32) error { return nil }
func (g *NoopGraph) Search(_ []float32, _ int) ([]string, []float32, error) { return nil, nil, nil }
func (g *NoopGraph) Serialize() ([]byte, error) { return nil, nil }
func (g *NoopGraph) Deserialize(_ []byte) error { return nil }
