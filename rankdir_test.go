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
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		return graph.Bounds()
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
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		inside := func(node *layout.Node, p layout.Vector) bool {
			const tolerance = 0.01
			return p.X > node.Left()-tolerance && p.X < node.Right()+tolerance &&
				p.Y > node.Top()-tolerance && p.Y < node.Bottom()+tolerance
		}
		if !inside(edge.From, edge.Path[0]) || !inside(edge.To, edge.Path[len(edge.Path)-1]) {
			t.Errorf("%v: path %v does not connect %v at %v and %v at %v", dir, edge.Path, edge.From, edge.From.Center, edge.To, edge.To.Center)
		}
		min, max := graph.Bounds()
		if pos := edge.LabelPos; pos.X < min.X || pos.X > max.X || pos.Y < min.Y || pos.Y > max.Y {
			t.Errorf("%v: label at %v outside the drawing %v-%v", dir, pos, min, max)
		}
	}
}
