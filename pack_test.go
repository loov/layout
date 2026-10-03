package layout_test

import (
	"testing"

	"github.com/loov/layout"
)

// TestPackEdgeEnds checks that with packed edge ends the main path of a
// sideways layout runs straight along the first end of its nodes, an
// edge padding below their tops, with further ends below it.
func TestPackEdgeEnds(t *testing.T) {
	graph := layout.NewDigraph()
	graph.RankDir = layout.LeftToRight
	graph.Splines = layout.SplinesOrtho
	graph.PackEdgeEnds = true
	main := graph.Edge("a", "b")
	graph.Edge("b", "c")
	side := graph.Edge("a", "c")
	if err := layout.HierarchicalWith(graph, layout.Options{Align: layout.AlignLeft}); err != nil {
		t.Fatal(err)
	}
	a := graph.Node("a")
	want := a.Top() + graph.EdgePadding
	for _, p := range main.Path {
		if p.Y != want {
			t.Errorf("a -> b at y %v, want straight at %v: %v", p.Y, want, main.Path)
			break
		}
	}
	if y := side.Path[0].Y; y <= want || y >= a.Bottom() {
		t.Errorf("a -> c leaves a at y %v, want below %v and inside a", y, want)
	}
}
