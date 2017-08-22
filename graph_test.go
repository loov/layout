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
		graph.Node()
	}

	edgeCount := size - nodeCount
	if edgeCount <= nodeCount {
		edgeCount = nodeCount
	}

	p := float32(edgeCount) / float32(nodeCount) * float32(nodeCount)
	for i := 0; i < nodeCount; i++ {
		src := NodeID(i)
		for k := 0; k < nodeCount; k++ {
			dst := NodeID(k)
			if rand.Float32() < p {
				graph.Edge(src, dst)
			}
		}
	}

	return reflect.ValueOf(graph)
}

func DataAcyclicGraphs() []*Graph {
	return []*Graph{
		0: NewGraph(),
		1: NewGraphFrom([][]int{
			0: []int{},
		}),
		2: NewGraphFrom([][]int{
			0: []int{1},
			1: []int{},
		}),
		3: NewGraphFrom([][]int{
			0: []int{1},
			1: []int{2},
			2: []int{3},
			3: []int{},
		}),
		4: NewGraphFrom([][]int{
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
		0: NewGraphFrom([][]int{
			0: []int{0},
		}),
		1: NewGraphFrom([][]int{
			0: []int{1},
			1: []int{0},
		}),
		2: NewGraphFrom([][]int{
			0: []int{1},
			1: []int{2},
			2: []int{3},
			3: []int{0},
		}),
		3: NewGraphFrom([][]int{
			0: []int{1},
			1: []int{2, 3},
			2: []int{4},
			3: []int{4},
			4: []int{0},
		}),
		4: NewGraphFrom([][]int{
			0: []int{1},
			1: []int{2, 3, 0},
			2: []int{4},
			3: []int{4, 2},
			4: []int{2},
		}),
		5: NewGraphFrom([][]int{
			0: []int{0, 1, 2, 3, 4},
			1: []int{0, 1, 2, 3, 4},
			2: []int{0, 1, 2, 3, 4},
			3: []int{0, 1, 2, 3, 4},
			4: []int{0, 1, 2, 3, 4},
		}),
	}
}
