package hnsw

// NoopGraph is a minimal graph implementation for tests.
type NoopGraph struct{}

// NewNoopGraph creates a new NoopGraph.
func NewNoopGraph() *NoopGraph {
	return &NoopGraph{}
}

// Insert is a no-op.
func (g *NoopGraph) Insert(_ string, _ []float32) error { return nil }

// Search returns no results.
func (g *NoopGraph) Search(_ []float32, _ int) ([]string, []float32, error) { return nil, nil, nil }

// Serialize returns an empty graph payload.
func (g *NoopGraph) Serialize() ([]byte, error) { return nil, nil }

// Deserialize is a no-op.
func (g *NoopGraph) Deserialize(_ []byte) error { return nil }
