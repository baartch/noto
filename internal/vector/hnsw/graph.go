package hnsw

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"math"
	"sort"
)

// Node represents an HNSW node.
type Node struct {
	ID     string
	Vector []float32
}

// Graph defines the HNSW operations needed by the vector index.
type Graph interface {
	Insert(id string, vector []float32) error
	Search(query []float32, k int) ([]string, []float32, error)
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// SimpleGraph is a lightweight cosine-similarity index used as a placeholder.
type SimpleGraph struct {
	Dim   int
	Nodes []Node
}

// NewSimpleGraph creates a SimpleGraph.
func NewSimpleGraph(dim int) *SimpleGraph {
	return &SimpleGraph{Dim: dim}
}

// Insert adds a node to the graph.
func (g *SimpleGraph) Insert(id string, vector []float32) error {
	if g.Dim == 0 {
		g.Dim = len(vector)
	}
	if len(vector) != g.Dim {
		return errors.New("hnsw: dim mismatch")
	}
	g.Nodes = append(g.Nodes, Node{ID: id, Vector: vector})
	return nil
}

// Search performs a linear cosine search (placeholder for HNSW).
func (g *SimpleGraph) Search(query []float32, k int) ([]string, []float32, error) {
	if len(g.Nodes) == 0 {
		return nil, nil, nil
	}
	if k <= 0 {
		return nil, nil, nil
	}
	if len(query) != g.Dim {
		return nil, nil, errors.New("hnsw: query dim mismatch")
	}
	results := make([]result, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		results = append(results, result{id: n.ID, score: cosine(query, n.Vector)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > k {
		results = results[:k]
	}
	ids := make([]string, 0, len(results))
	scores := make([]float32, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.id)
		scores = append(scores, r.score)
	}
	return ids, scores, nil
}

// Serialize writes the graph to bytes.
func (g *SimpleGraph) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(g); err != nil {
		return nil, fmt.Errorf("hnsw: serialize: %w", err)
	}
	return buf.Bytes(), nil
}

// Deserialize loads the graph from bytes.
func (g *SimpleGraph) Deserialize(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(g); err != nil {
		return fmt.Errorf("hnsw: deserialize: %w", err)
	}
	return nil
}

type result struct {
	id    string
	score float32
}

func cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
