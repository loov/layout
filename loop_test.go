package layout_test

import (
	"testing"

	"github.com/loov/layout"
)

// TestLoopLeavesHorizontally checks that a self-loop leaves and returns
// horizontally from the outline, so that drawings on a grid don't get a
// step at its ends.
func TestLoopLeavesHorizontally(t *testing.T) {
	for _, shape := range []layout.Shape{layout.Ellipse, layout.Circle, layout.Box} {
		graph := layout.NewDigraph()
		graph.Node("a").Shape = shape
		loop := graph.Edge("a", "a")
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		p := loop.Path
		if len(p) != 4 || p[0].Y != p[1].Y || p[2].Y != p[3].Y {
			t.Errorf("%s: loop path %v, want horizontal ends", shape, p)
		}
	}
}
