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

// TestAlignChains checks that a virtual node between two nodes that
// don't line up moves in line with one of them, so that an orthogonal
// edge through it jogs once.
func TestAlignChains(t *testing.T) {
	graph := NewGraph()
	a, b, c := graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(a, b)
	graph.AddEdge(a, c)
	graph.AddEdge(c, b)
	for _, node := range graph.Nodes {
		node.Radius = Vector{X: 20, Y: 10}
	}

	Rank(graph)
	AddVirtuals(graph)
	PositionInitial(graph)
	v := a.Out[0]
	if !v.IsChainLink() {
		t.Fatal("no virtual node between a and b")
	}
	// c out of the way, the chain bent at v
	c.Center.X = -1000
	a.Center.X, v.Center.X, b.Center.X = 0, 50, 100

	alignChains(graph)
	if v.Center.X != a.Center.X && v.Center.X != b.Center.X {
		t.Errorf("virtual node at %v, want in line with 0 or 100", v.Center.X)
	}
}

func TestAlignChainsKeepsPackedEnds(t *testing.T) {
	graph := NewGraph()
	a, b, c := graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(a, c)
	graph.AddEdge(a, b)
	graph.AddEdge(c, b)
	graph.EndGap = 10
	for _, node := range graph.Nodes {
		node.Radius = Vector{X: 20, Y: 10}
		node.EndRoom = 30
	}

	Rank(graph)
	AddVirtuals(graph)
	PositionInitial(graph)
	var v *Node
	for _, dst := range a.Out {
		if dst.IsChainLink() {
			v = dst
		}
	}
	if v == nil {
		t.Fatal("no virtual node between a and b")
	}
	from, _ := endOffsets(graph)
	slot := from[[2]ID{a.ID, v.ID}]
	if slot == 0 {
		t.Fatal("chain end not packed away from the anchor")
	}
	// v in line with its packed end on a, b elsewhere
	a.Center.X, b.Center.X = 0, 200
	v.Center.X = slot

	alignChains(graph)
	if v.Center.X != slot {
		t.Errorf("virtual node moved to %v, want kept in line with its packed end at %v", v.Center.X, slot)
	}
}
