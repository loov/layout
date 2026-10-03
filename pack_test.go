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
	l, err := layout.Hierarchical(graph, layout.Options{Align: layout.AlignLeft})
	if err != nil {
		t.Fatal(err)
	}
	a := l.Node(graph.Node("a"))
	want := a.Top() + l.Graph.EdgePadding
	for _, p := range l.Edge(main).Path {
		if p.Y != want {
			t.Errorf("a -> b at y %v, want straight at %v: %v", p.Y, want, l.Edge(main).Path)
			break
		}
	}
	if y := l.Edge(side).Path[0].Y; y <= want || y >= a.Bottom() {
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
			l, err := layout.Hierarchical(graph, layout.Options{})
			if err != nil {
				t.Fatal(err)
			}
			size := l.Node(a).Size
			if size.X != size.Y {
				t.Errorf("%v run %d: circle size %v", dir, run, size)
			}
			if run == 0 {
				first = size
			} else if size != first {
				t.Errorf("%v run %d: size %v, was %v", dir, run, size, first)
			}
		}
	}
}
