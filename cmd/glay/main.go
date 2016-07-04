package main

import (
	"fmt"
	"os"

	"github.com/kr/pretty"
	"github.com/loov/glay"
)

func main() {
	graph := glay.NewGraph()

	nodelabel := map[glay.NodeID]string{}
	a := graph.Node(glay.NodeInfo{HalfSize: glay.Vector{10, 10}})
	nodelabel[a] = "A"
	b := graph.Node(glay.NodeInfo{HalfSize: glay.Vector{10, 10}})
	nodelabel[b] = "B"
	c := graph.Node(glay.NodeInfo{HalfSize: glay.Vector{10, 10}})
	nodelabel[c] = "C"
	e := graph.Node(glay.NodeInfo{HalfSize: glay.Vector{10, 10}})
	nodelabel[e] = "E"
	f := graph.Node(glay.NodeInfo{HalfSize: glay.Vector{10, 10}})
	nodelabel[f] = "F"
	h := graph.Node(glay.NodeInfo{HalfSize: glay.Vector{10, 10}})
	nodelabel[h] = "H"

	graph.Edge(a, b)
	graph.Edge(a, c)

	graph.Edge(b, e)
	graph.Edge(c, f)
	graph.Edge(c, e)
	//graph.Edge(b, f)

	graph.Edge(e, h)
	graph.Edge(f, h)

	graph.Edge(a, h)

	glay.BreakCycles(graph)
	glay.AssignRanks(graph)
	glay.CreateDummies(graph)
	glay.AssignPositions(graph)

	pretty.Println(graph)

	out, _ := os.Create("~out.svg")
	defer out.Close()

	fmt.Fprintf(out, "<svg xmlns='http://www.w3.org/2000/svg'>\n")
	defer fmt.Fprintf(out, "</svg>\n")

	fmt.Fprintf(out, "<g>\n")
	defer fmt.Fprintf(out, "</g>\n")

	for nid := range graph.Nodes {
		n := &graph.Nodes[nid]
		fmt.Fprintf(out, "<circle cx='%f' cy='%f' r='%f' fill='white' stroke='black'>", n.Center.X, n.Center.Y, n.HalfSize.X)
		fmt.Fprintf(out, "</circle>\n")
	}

	for eid := range graph.Edges {
		e := &graph.Edges[eid]
		s, d := &graph.Nodes[e.Source], &graph.Nodes[e.Destination]
		if d.Rank-s.Rank > 1 {
			continue
		}

		fmt.Fprintf(out, "<line x1='%f' y1='%f' x2='%f' y2='%f' stroke='black'>",
			s.Center.X, s.Center.Y, d.Center.X, d.Center.Y)
		fmt.Fprintf(out, "</line>\n")
	}
}
