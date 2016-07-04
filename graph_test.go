package glay

import "testing"

func TestAlgorithms(t *testing.T) {
	g := NewGraph()

	a := g.Node(NodeInfo{})
	b := g.Node(NodeInfo{})
	c := g.Node(NodeInfo{})
	e := g.Node(NodeInfo{})
	f := g.Node(NodeInfo{})
	h := g.Node(NodeInfo{})

	g.Edge(a, b)
	g.Edge(a, c)

	g.Edge(b, e)
	g.Edge(c, f)
	g.Edge(c, e)
	g.Edge(b, f)

	g.Edge(e, h)
	g.Edge(f, h)
	g.Edge(a, h)

	AssignRanks(g)

	isrank := func(id NodeID, rank int) {
		if g.Nodes[id].Rank != rank {
			t.Errorf("invalid rank for %v; %v != %v ", id, g.Nodes[id].Rank, rank)
		}
	}

	isrank(a, 0)
	isrank(b, 1)
	isrank(c, 1)
	isrank(e, 2)
	isrank(f, 2)
	isrank(h, 3)

	CreateDummies(g)
}
