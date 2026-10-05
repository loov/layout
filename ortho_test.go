package layout

import (
	"slices"
	"testing"
)

func TestSeparate(t *testing.T) {
	for _, tc := range []struct {
		want   []Length
		weight []float64
		out    []Length
	}{
		{[]Length{0, 10, 20}, []float64{1, 1, 1}, []Length{0, 10, 20}},
		{[]Length{0, 0}, []float64{1, 1}, []Length{-5, 5}},
		{[]Length{0, 0}, []float64{1, 3}, []Length{-7.5, 2.5}},
		{[]Length{-5, 0, 5}, []float64{1, 1, 1}, []Length{-10, 0, 10}},
		{[]Length{90, 100}, []float64{1, 1}, []Length{85, 95}}, // clamped to hi
	} {
		got := separate(tc.want, tc.weight, 10, -100, 95)
		if !slices.Equal(got, tc.out) {
			t.Errorf("separate(%v, %v) = %v, want %v", tc.want, tc.weight, got, tc.out)
		}
	}
}

// TestOrthoTrackCycle routes three edges between two ranks that each enter
// the channel where another leaves it, around a cycle, so that no order of
// single tracks keeps their vertical runs apart: one of them must step
// over on a second track.
func TestOrthoTrackCycle(t *testing.T) {
	def := NewDigraph()
	at := map[string]Vector{"a": {216, 0}, "b": {408, 0}, "c": {344, 0}, "x": {408, 100}, "y": {344, 100}, "z": {216, 100}}
	for _, e := range [][2]string{{"a", "x"}, {"b", "y"}, {"c", "z"}} {
		def.Edge(e[0], e[1])
	}
	graph := newWorkGraph(def)
	for _, node := range graph.Nodes {
		node.Center, node.Radius = at[node.ID], Vector{10, 10}
		node.Shape = Box
	}
	for _, edge := range graph.Edges {
		edge.Path = []Vector{edge.From.BottomCenter(), edge.To.TopCenter()}
	}
	rows := [][2]Length{{-10, 10}, {90, 110}}
	orthoEdges(graph, rows, 10, false)

	// vertical runs of different edges at one x must not overlap
	type run struct {
		edge   *ledge
		x      Length
		y0, y1 Length
	}
	var runs []run
	for _, edge := range graph.Edges {
		for i := 0; i+1 < len(edge.Path); i++ {
			a, b := edge.Path[i], edge.Path[i+1]
			if a.X == b.X && a.Y != b.Y {
				runs = append(runs, run{edge, a.X, min(a.Y, b.Y), max(a.Y, b.Y)})
			}
		}
	}
	for i, a := range runs {
		for _, b := range runs[i+1:] {
			if a.edge != b.edge && absLength(a.x-b.x) < 5 && a.y0 < b.y1-0.5 && b.y0 < a.y1-0.5 {
				t.Errorf("%v and %v run down x=%v together, %v-%v and %v-%v", a.edge, b.edge, a.x, a.y0, a.y1, b.y0, b.y1)
			}
		}
	}
}
