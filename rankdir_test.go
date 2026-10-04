package layout_test

import (
	"math"
	"testing"

	"github.com/loov/layout"
)

// TestRankDirMargins checks that flipped layouts start at the same margins
// as their unflipped counterparts.
func TestRankDirMargins(t *testing.T) {
	bounds := func(dir layout.RankDir) (min, max layout.Vector) {
		graph := layout.NewDigraph()
		graph.RankDir = dir
		graph.Edge("a", "this is a very very very long label node")
		l, err := layout.Hierarchical(graph, layout.Options{})
		if err != nil {
			t.Fatal(err)
		}
		return l.Bounds()
	}
	near := func(a, b layout.Vector) bool {
		return math.Abs(float64(a.X-b.X)) < 1e-3 && math.Abs(float64(a.Y-b.Y)) < 1e-3
	}
	for _, pair := range [][2]layout.RankDir{
		{layout.LeftToRight, layout.RightToLeft},
		{layout.TopToBottom, layout.BottomToTop},
	} {
		wantMin, wantMax := bounds(pair[0])
		gotMin, gotMax := bounds(pair[1])
		if !near(gotMin, wantMin) || !near(gotMax, wantMax) {
			t.Errorf("%v bounds %v-%v, want %v-%v like %v", pair[1], gotMin, gotMax, wantMin, wantMax, pair[0])
		}
	}
}

// TestRankDirDuplicateEdge checks that an edge listed twice is mapped
// out of the layout frame once, so that it still ends at its nodes.
func TestRankDirDuplicateEdge(t *testing.T) {
	for _, dir := range []layout.RankDir{layout.LeftToRight, layout.RightToLeft, layout.BottomToTop} {
		graph := layout.NewDigraph()
		graph.RankDir = dir
		graph.Edge("c", "d") // a component before, so that a -> b is shifted
		edge := graph.Edge("a", "b")
		edge.Label = "label"
		graph.AddEdge(edge)
		l, err := layout.Hierarchical(graph, layout.Options{})
		if err != nil {
			t.Fatal(err)
		}
		inside := func(node layout.NodeBox, p layout.Vector) bool {
			const tolerance = 0.01
			return p.X > node.Left()-tolerance && p.X < node.Right()+tolerance &&
				p.Y > node.Top()-tolerance && p.Y < node.Bottom()+tolerance
		}
		from, to, path := l.Node(edge.From), l.Node(edge.To), l.Edge(edge)
		if !inside(from, path.Path[0]) || !inside(to, path.Path[len(path.Path)-1]) {
			t.Errorf("%v: path %v does not connect %v at %v and %v at %v", dir, path.Path, edge.From, from.Center, edge.To, to.Center)
		}
		min, max := l.Bounds()
		if pos := path.LabelCenter; pos.X < min.X || pos.X > max.X || pos.Y < min.Y || pos.Y > max.Y {
			t.Errorf("%v: label at %v outside the drawing %v-%v", dir, pos, min, max)
		}
	}
}

// TestRankDirRepeat checks that laying out a graph again gives the same
// label positions, unlabeled edges included.
func TestRankDirRepeat(t *testing.T) {
	for _, dir := range []layout.RankDir{layout.LeftToRight, layout.RightToLeft, layout.BottomToTop} {
		graph := layout.NewDigraph()
		graph.RankDir = dir
		graph.Edge("a", "b")
		graph.Edge("c", "d").Label = "label"
		graph.Edge("e", "f")
		var first []layout.Vector
		for run := range 3 {
			l, err := layout.Hierarchical(graph, layout.Options{})
			if err != nil {
				t.Fatal(err)
			}
			for i, edge := range graph.Edges {
				pos := l.Edges[i].LabelCenter
				if run == 0 {
					first = append(first, pos)
				} else if pos != first[i] {
					t.Errorf("%v run %d: %v label at %v, was %v", dir, run, edge, pos, first[i])
				}
			}
		}
	}
}

// TestTextCircleDefault checks that a circle from the graph's default
// shape is laid out for text like a node's own circle: as an ellipse, so
// that a sideways node isn't as wide as its packed edges make it tall.
func TestTextCircleDefault(t *testing.T) {
	for _, own := range []bool{true, false} {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		if !own {
			graph.Shape = layout.Circle
		}
		for _, id := range []string{"a", "b", "c", "d"} {
			if own {
				graph.Node(id).Shape = layout.Circle
			}
		}
		graph.Edge("a", "b")
		graph.Edge("a", "c")
		graph.Edge("a", "d")
		l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
		if err != nil {
			t.Fatal(err)
		}
		if box := l.Node(graph.Nodes[0]); box.Shape != layout.Ellipse {
			t.Errorf("own circle %v: laid out as %v, want ellipse", own, box.Shape)
		}
	}
}
