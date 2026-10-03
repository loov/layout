package layout_test

import (
	"testing"

	"github.com/loov/layout"
)

// TestAlign checks that left and right alignment put a parent over its
// first and last child, and that sideways layouts turn left into top.
func TestAlign(t *testing.T) {
	for _, tc := range []struct {
		align    layout.Align
		sideways bool
		over     string // the child the parent lines up with
	}{
		{layout.AlignLeft, false, "b"},
		{layout.AlignRight, false, "d"},
		{layout.AlignLeft, true, "b"},
		{layout.AlignRight, true, "d"},
	} {
		graph := layout.NewDigraph()
		graph.Edge("a", "b")
		graph.Edge("a", "c")
		graph.Edge("a", "d")
		if tc.sideways {
			graph.RankDir = layout.LeftToRight
		}
		l, err := layout.Hierarchical(graph, layout.Options{Align: tc.align})
		if err != nil {
			t.Fatal(err)
		}
		parent, child := l.Node(graph.Node("a")).Center, l.Node(graph.Node(tc.over)).Center
		along, childAlong := parent.X, child.X
		if tc.sideways {
			along, childAlong = parent.Y, child.Y
		}
		if along != childAlong {
			t.Errorf("align %v sideways=%v: a at %v, want in line with %s at %v", tc.align, tc.sideways, parent, tc.over, child)
		}
	}
}

// TestAlignFanIn checks that left and right alignment put a node under
// its first or last parent, not the median one.
func TestAlignFanIn(t *testing.T) {
	for align, over := range map[layout.Align]string{layout.AlignLeft: "a", layout.AlignRight: "c"} {
		graph := layout.NewDigraph()
		graph.Edge("a", "d")
		graph.Edge("b", "d")
		graph.Edge("c", "d")
		l, err := layout.Hierarchical(graph, layout.Options{Align: align})
		if err != nil {
			t.Fatal(err)
		}
		if d, p := l.Node(graph.Node("d")).Center.X, l.Node(graph.Node(over)).Center.X; d != p {
			t.Errorf("align %v: d at x %v, want under %s at %v", align, d, over, p)
		}
	}
}
