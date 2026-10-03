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
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range graph.Edges {
		path := l.Edge(edge).Path
		for i := 0; i+1 < len(path); i++ {
			p, q := path[i], path[i+1]
			for _, node := range graph.Nodes {
				if node == edge.From || node == edge.To {
					continue
				}
				box := l.Node(node)
				const steps = 64
				for s := range steps + 1 {
					f := layout.Length(s) / steps
					x, y := p.X+(q.X-p.X)*f, p.Y+(q.Y-p.Y)*f
					if x > box.Left() && x < box.Right() && y > box.Top() && y < box.Bottom() {
						t.Fatalf("%v passes through %v: %v", edge, node, path)
					}
				}
			}
		}
	}
}

// TestFlatArcsStack checks that overlapping arcs of flat edges over the
// same rank run at different heights instead of along one line.
func TestFlatArcsStack(t *testing.T) {
	graph := layout.NewDigraph()
	graph.SameRank = [][]*layout.Node{{graph.Node("a"), graph.Node("b"), graph.Node("c"), graph.Node("d")}}
	for _, e := range [][2]string{{"a", "d"}, {"c", "a"}, {"b", "c"}, {"d", "b"}} {
		graph.Edge(e[0], e[1])
	}
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	heights := map[layout.Length]string{}
	for _, edge := range graph.Edges {
		path := l.Edge(edge).Path
		if len(path) != 4 {
			continue // adjacent nodes, no arc
		}
		y := path[1].Y
		if other, ok := heights[y]; ok {
			t.Errorf("arcs of %v and %s both run at y %v", edge, other, y)
		}
		heights[y] = edge.String()
	}
}
