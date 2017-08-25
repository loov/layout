package hier

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
	DecycleDefault(graph)

	// assign ranks to nodes

	// order nodes in each rank

	// position nodes on a canvas

	// output graph
}
