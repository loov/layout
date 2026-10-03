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
