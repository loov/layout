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

// TestPackEdgeEndsRepeat checks that the room made for packed edge ends
// keeps circles round and does not carry over to the next layout.
func TestPackEdgeEndsRepeat(t *testing.T) {
	for _, dir := range []layout.RankDir{layout.TopToBottom, layout.LeftToRight} {
		graph := layout.NewDigraph()
		graph.RankDir = dir
		graph.Splines = layout.SplinesOrtho
		graph.PackEdgeEnds = true
		a := graph.Node("a")
		a.Shape = layout.Circle
		for _, to := range []string{"b", "c", "d", "e", "f", "g"} {
			graph.Edge("a", to)
		}
		var first layout.Vector
		for run := range 3 {
			if err := layout.Hierarchical(graph); err != nil {
				t.Fatal(err)
			}
			if a.Radius.X != a.Radius.Y {
				t.Errorf("%v run %d: circle radius %v", dir, run, a.Radius)
			}
			if run == 0 {
				first = a.Radius
			} else if a.Radius != first {
				t.Errorf("%v run %d: radius %v, was %v", dir, run, a.Radius, first)
			}
		}
	}
}
