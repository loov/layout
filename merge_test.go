package layout_test

import (
	"testing"

	"github.com/loov/layout"
)

// TestMergeEdges checks which ends merge: those leaving or entering a
// node together, of edges that look the same and have no label, and of
// an edge only at one end.
func TestMergeEdges(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Splines = layout.SplinesOrtho
	graph.MergeEdges = true
	ab, ac := graph.Edge("a", "b"), graph.Edge("a", "c")
	ad := graph.Edge("a", "d")
	ad.ArrowHead = layout.ArrowDot // looks different
	ae := graph.Edge("a", "e")
	ae.Label = "label"
	// b -> f would merge with a's start and f's end: it merges at neither
	bf, cf := graph.Edge("b", "f"), graph.Edge("c", "f")
	bg, bh := graph.Edge("b", "g"), graph.Edge("b", "h")
	gf := graph.Edge("g", "f")
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	merged := func(edge *layout.Edge) [2]int { return l.Edge(edge).Merged }
	start := merged(ab)[0]
	if start == 0 || merged(ac)[0] != start || merged(ab)[1] != 0 {
		t.Errorf("a -> b and a -> c: %v %v, want one start group", merged(ab), merged(ac))
	}
	if l.Edge(ab).Path[0] != l.Edge(ac).Path[0] {
		t.Errorf("a -> b and a -> c start at %v and %v, want together", l.Edge(ab).Path[0], l.Edge(ac).Path[0])
	}
	for _, edge := range []*layout.Edge{ad, ae, bf} {
		if m := merged(edge); m != [2]int{} {
			t.Errorf("%v merged %v, want apart", edge, m)
		}
	}
	if m := merged(bg); m[0] == 0 || m[0] != merged(bh)[0] || m[0] == start {
		t.Errorf("b -> g and b -> h: %v %v, want a start group of their own", m, merged(bh))
	}
	if m := merged(cf); m[1] == 0 || m[1] != merged(gf)[1] {
		t.Errorf("c -> f and g -> f: %v %v, want one end group", m, merged(gf))
	}
}
