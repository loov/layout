package hier

import "fmt"

func Example() {
	// create a new graph
	graph := NewGraph()
	a, b, c, d := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()

	graph.AddEdge(a, b)
	graph.AddEdge(a, c)
	graph.AddEdge(b, d)
	graph.AddEdge(c, d)
	graph.AddEdge(d, a)

	// remove cycles from the graph
	decycledGraph := DefaultDecycle(graph)

	// assign nodes to ranks
	rankedGraph := DefaultRank(decycledGraph)

	// create virtual nodes
	filledGraph := DefaultAddVirtuals(rankedGraph)

	// order nodes in ranks
	orderedGraph := DefaultOrderRanks(filledGraph)

	for _, node := range orderedGraph.Nodes {
		node.Radius.X = 10
		node.Radius.Y = 10
	}

	// position nodes
	positionedGraph := DefaultPosition(orderedGraph)

	for _, node := range positionedGraph.Nodes {
		fmt.Println(node.ID, node.Center.X, node.Center.Y)
	}
}
