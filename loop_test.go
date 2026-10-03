package layout_test

import (
	"slices"
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
		l, err := layout.Hierarchical(graph, layout.Options{})
		if err != nil {
			t.Fatal(err)
		}
		p := l.Edge(loop).Path
		if len(p) != 4 || p[0].Y != p[1].Y || p[2].Y != p[3].Y {
			t.Errorf("%s: loop path %v, want horizontal ends", shape, p)
		}
	}
}

// TestLoopNodeLinesUp checks that a node with a self-loop lines up with
// its neighbors: the room reserved for the loop doesn't shift it.
func TestLoopNodeLinesUp(t *testing.T) {
	for _, align := range []layout.Align{layout.AlignBalanced, layout.AlignLeft, layout.AlignRight} {
		graph := layout.NewDigraph()
		graph.Edge("a", "b")
		graph.Edge("b", "b")
		graph.Edge("b", "c")
		l, err := layout.Hierarchical(graph, layout.Options{Align: align})
		if err != nil {
			t.Fatal(err)
		}
		a, b, c := l.Node(graph.Node("a")).Center.X, l.Node(graph.Node("b")).Center.X, l.Node(graph.Node("c")).Center.X
		if a != b || b != c {
			t.Errorf("align %v: a, b, c at x %v, %v, %v, want one line", align, a, b, c)
		}
	}
}

// TestLoopsApart checks that several self-loops of a node take routes of
// their own, with labels apart, in hierarchical and force layouts.
func TestLoopsApart(t *testing.T) {
	for name, run := range algorithms {
		graph := layout.NewDigraph()
		for _, label := range []string{"first", "second"} {
			edge := layout.NewEdge(graph.Node("a"), graph.Node("a"))
			edge.Label = label
			graph.AddEdge(edge)
		}
		l, err := run(graph)
		if err != nil {
			t.Fatal(err)
		}
		a, b := l.Edges[0], l.Edges[1]
		if slices.Equal(a.Path, b.Path) {
			t.Errorf("%s: loops share the route %v", name, a.Path)
		}
		if gap := absLength(a.LabelCenter.Y - b.LabelCenter.Y); gap < (a.LabelSize.Y+b.LabelSize.Y)/2 {
			t.Errorf("%s: labels at %v and %v overlap", name, a.LabelCenter, b.LabelCenter)
		}
	}
}

func absLength(v layout.Length) layout.Length { return max(v, -v) }

// algorithms lays out a graph with each algorithm and default options
var algorithms = map[string]func(*layout.Graph) (*layout.Layout, error){
	"hierarchical": func(graph *layout.Graph) (*layout.Layout, error) {
		return layout.Hierarchical(graph, layout.Options{})
	},
	"force": func(graph *layout.Graph) (*layout.Layout, error) {
		return layout.Force(graph, layout.ForceOptions{})
	},
}
