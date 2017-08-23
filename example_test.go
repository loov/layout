package layout_test

import "github.com/loov/layout"

func ExampleLayout() {
	// create a new graph
	graph := layout.NewGraph()
	a, b, c, d := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()

	graph.AddEdge(a, b)
	graph.AddEdge(a, c)
	graph.AddEdge(b, d)
	graph.AddEdge(c, d)
	graph.AddEdge(d, a)

	// remove cycles from the graph
	layout.DecycleDefault(graph)

	// assign ranks to nodes

	// order nodes in each rank

	// position nodes on a canvas

	// output graph
}
