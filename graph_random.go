package layout

import (
	"math/rand"
	"reflect"
)

// GenerateRandomGraph creates a graph with n nodes and a P(src, dst) == p
func GenerateRandomGraph(n int, p float64, rand *rand.Rand) *Graph {
	graph := NewGraph()
	for i := 0; i < n; i++ {
		graph.AddNode()
	}

	for _, src := range graph.Nodes {
		for _, dst := range graph.Nodes {
			if rand.Float64() < p {
				graph.AddEdge(src, dst)
			}
		}
	}

	return graph
}

// GenerateRegularGraph creates a circular graph
func GenerateRegularGraph(n, connections int) *Graph {
	graph := NewGraph()
	for i := 0; i < n; i++ {
		graph.AddNode()
	}

	for i := 0; i < n; i++ {
		for k := i + 1; k < i+connections+1; k++ {
			graph.AddEdge(graph.Nodes[i], graph.Nodes[k%n])
		}
	}
	return graph
}

// Generate implements quick.Generator interface
func (_ *Graph) Generate(rand *rand.Rand, size int) reflect.Value {
	switch rand.Intn(4) {
	case 0:
		return reflect.ValueOf(GenerateRandomGraph(size, 0.1, rand))
	case 1:
		return reflect.ValueOf(GenerateRandomGraph(size, 0.3, rand))
	case 2:
		return reflect.ValueOf(GenerateRandomGraph(size, 0.7, rand))
	case 3:
		return reflect.ValueOf(GenerateRegularGraph(size, rand.Intn(size)))
	}

	return reflect.ValueOf(GenerateRandomGraph(size, 0.5, rand))
}
