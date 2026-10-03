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
		if err := layout.HierarchicalWith(graph, layout.Options{Align: tc.align}); err != nil {
			t.Fatal(err)
		}
		parent, child := graph.Node("a").Center, graph.Node(tc.over).Center
		along, childAlong := parent.X, child.X
		if tc.sideways {
			along, childAlong = parent.Y, child.Y
		}
		if along != childAlong {
			t.Errorf("align %v sideways=%v: a at %v, want in line with %s at %v", tc.align, tc.sideways, parent, tc.over, child)
		}
	}
}
