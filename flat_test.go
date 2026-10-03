package layout_test

import (
	"testing"

	"github.com/loov/layout"
)

// TestFlatEdgesAvoidNodes checks that edges within a rank arc over the
// nodes between their ends, also when they run right to left.
func TestFlatEdgesAvoidNodes(t *testing.T) {
	graph := layout.NewDigraph()
	graph.SameRank = [][]*layout.Node{{graph.Node("a"), graph.Node("b"), graph.Node("c"), graph.Node("d")}}
	// a cycle, so that some edge has to run right to left
	graph.Edge("a", "d")
	graph.Edge("c", "a")
	graph.Edge("b", "c")
	graph.Edge("d", "b")
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	for _, edge := range graph.Edges {
		for i := 0; i+1 < len(edge.Path); i++ {
			p, q := edge.Path[i], edge.Path[i+1]
			for _, node := range graph.Nodes {
				if node == edge.From || node == edge.To {
					continue
				}
				const steps = 64
				for s := range steps + 1 {
					f := layout.Length(s) / steps
					x, y := p.X+(q.X-p.X)*f, p.Y+(q.Y-p.Y)*f
					if x > node.Left() && x < node.Right() && y > node.Top() && y < node.Bottom() {
						t.Fatalf("%v passes through %v: %v", edge, node, edge.Path)
					}
				}
			}
		}
	}
}
