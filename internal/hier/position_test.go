package hier

import "testing"

// TestPositionAlignPorts checks that with AlignPorts the ends Ports puts
// on two nodes line up, rather than the nodes. The lower node fits under
// the upper one either way, so the drawing is as wide.
func TestPositionAlignPorts(t *testing.T) {
	for _, align := range []bool{false, true} {
		graph := NewGraph()
		a, b := graph.AddNode(), graph.AddNode()
		graph.AddEdge(a, b)
		a.Radius, b.Radius = Vector{X: 50, Y: 10}, Vector{X: 20, Y: 10}
		graph.Ports = map[[2]ID][2]float32{{a.ID, b.ID}: {-0.2, 0}}
		graph.AlignPorts = align

		Decycle(graph)
		Rank(graph)
		AddVirtuals(graph)
		OrderRanks(graph)
		Position(graph, false, false, Balanced)

		// the ends at -20 on a and the middle of b
		got := (a.Center.X - 20) - b.Center.X
		want := float32(-20)
		if align {
			want = 0
		}
		if abs(got-want) > 1 {
			t.Errorf("align %v: ends %v apart, want %v", align, got, want)
		}
	}
}
