package layout

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestIsCyclic(t *testing.T) {
	for i, graph := range DataAcyclicGraphs() {
		if graph.IsCyclic() {
			t.Errorf("A%d: expected IsCyclic = false", i)
		}
		if err := graph.CheckErrors(); err != nil {
			t.Errorf("A%d: %v", i, err)
		}
	}

	for i, graph := range DataCyclicGraphs() {
		if !graph.IsCyclic() {
			t.Errorf("B%d: expected IsCyclic = true", i)
		}
		if err := graph.CheckErrors(); err != nil {
			t.Errorf("A%d: %v", i, err)
		}
	}
}

func (_ *Graph) Generate(rand *rand.Rand, size int) reflect.Value {
	graph := NewGraph()
	if size == 0 {
		return reflect.ValueOf(graph)
	}

	nodeCount := int(math.Sqrt(float64(size)))
	if nodeCount < 0 {
		nodeCount = 1
	}
	for i := 0; i < nodeCount; i++ {
		graph.AddNode()
	}

	edgeCount := size - nodeCount
	if edgeCount <= nodeCount {
		edgeCount = nodeCount
	}

	// p := float32(edgeCount) / float32(nodeCount) * float32(nodeCount)
	p := float32(0.5)
	for i := 0; i < nodeCount; i++ {
		src := ID(i)
		for k := 0; k < nodeCount; k++ {
			dst := ID(k)
			if rand.Float32() < p {
				graph.AddEdge(graph.Nodes[src], graph.Nodes[dst])
			}
		}
	}

	return reflect.ValueOf(graph)
}

func DataAcyclicGraphs() []*Graph {
	return []*Graph{
		0: NewGraph(),
		1: NewGraphFromEdgeList([][]int{
			0: []int{},
		}),
		2: NewGraphFromEdgeList([][]int{
			0: []int{1},
			1: []int{},
		}),
		3: NewGraphFromEdgeList([][]int{
			0: []int{1},
			1: []int{2},
			2: []int{3},
			3: []int{},
		}),
		4: NewGraphFromEdgeList([][]int{
			0: []int{1},
			1: []int{2, 3},
			2: []int{4},
			3: []int{4},
			4: []int{},
		}),
	}
}

func DataCyclicGraphs() []*Graph {
	return []*Graph{
		0: NewGraphFromEdgeList([][]int{
			0: []int{0},
		}),
		1: NewGraphFromEdgeList([][]int{
			0: []int{1},
			1: []int{0},
		}),
		2: NewGraphFromEdgeList([][]int{
			0: []int{1},
			1: []int{2},
			2: []int{3},
			3: []int{0},
		}),
		3: NewGraphFromEdgeList([][]int{
			0: []int{1},
			1: []int{2, 3},
			2: []int{4},
			3: []int{4},
			4: []int{0},
		}),
		4: NewGraphFromEdgeList([][]int{
			0: []int{1},
			1: []int{2, 3, 0},
			2: []int{4},
			3: []int{4, 2},
			4: []int{2},
		}),
		5: NewGraphFromEdgeList([][]int{
			0: []int{0, 1, 2, 3, 4},
			1: []int{0, 1, 2, 3, 4},
			2: []int{0, 1, 2, 3, 4},
			3: []int{0, 1, 2, 3, 4},
			4: []int{0, 1, 2, 3, 4},
		}),
		6: NewGraphFromEdgeList([][]int{
			0: []int{0, 1, 2, 4, 5},
			1: []int{0, 2, 3},
			2: []int{0, 1, 4, 5, 6},
			3: []int{0, 3, 4},
			4: []int{0, 1, 2, 3, 4, 5},
			5: []int{0, 1, 2},
			6: []int{0, 6},
		}),
		7: NewGraphFromEdgeList([][]int{
			0: []int{1, 2, 3, 4},
			1: []int{1, 5},
			2: []int{1},
			3: []int{0, 1, 2, 3},
			4: []int{0, 2},
			5: []int{0, 1, 2, 6},
			6: []int{1, 3, 4},
		}),
		8: NewGraphFromEdgeList([][]int{
			0: []int{1, 2, 3, 4, 5, 6},
			1: []int{0, 1, 2, 3, 4, 5, 6},
			2: []int{1},
			3: []int{0, 3, 4, 5, 6},
			4: []int{0, 1, 2, 3, 4, 5, 6},
			5: []int{0, 1, 2, 5, 6},
			6: []int{0, 1, 2, 3, 4, 5, 6},
		}),
	}
}
