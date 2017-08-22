package layout

import (
	"strconv"
	"testing"
	"testing/quick"
)

func TestDecycleOutdegree(t *testing.T)  { testDecycle(t, DecycleOutdegree) }
func TestDecycleDepthFirst(t *testing.T) { testDecycle(t, DecycleDepthFirst) }
func TestDecycleOrder(t *testing.T)      { testDecycle(t, DecycleOrder) }

func countLinks(graph *Graph) int {
	edges := make(dagEdgeTable)
	for _, node := range graph.Nodes {
		for _, out := range node.Out {
			if out != node.ID {
				edges.Include(node.ID, out)
			}
		}
	}
	return len(edges)
}

func tryDecycle(t *testing.T, graph *Graph, decycle func(*Graph)) {
	t.Helper()

	beforeCount := countLinks(graph)
	decycle(graph)

	printEdges := false
	if err := graph.CheckErrors(); err != nil {
		t.Errorf("got errors: %v", err)
		printEdges = true
	}
	if graph.IsCyclic() {
		t.Errorf("got cycles")
		printEdges = true
	}

	afterCount := countLinks(graph)
	if beforeCount != afterCount {
		t.Errorf("too many edges removed %v -> %v", beforeCount, afterCount)
		printEdges = true
	}

	if printEdges {
		t.Log("edge table: \n" + graph.EdgeTableString())
	}
}

func testDecycle(t *testing.T, decycle func(*Graph)) {
	for i, graph := range DataAcyclicGraphs() {
		t.Run("A"+strconv.Itoa(i), func(t *testing.T) {
			tryDecycle(t, graph, decycle)
		})
	}
	for i, graph := range DataCyclicGraphs() {
		t.Run("B"+strconv.Itoa(i), func(t *testing.T) {
			tryDecycle(t, graph, decycle)
		})
	}
}

func TestRandomDecycleOutdegree(t *testing.T)  { testDecycleRandom(t, DecycleOutdegree) }
func TestRandomDecycleDepthFirst(t *testing.T) { testDecycleRandom(t, DecycleDepthFirst) }
func TestRandomDecycleOrder(t *testing.T)      { testDecycleRandom(t, DecycleOrder) }

func testDecycleRandom(t *testing.T, decycle func(*Graph)) {
	noCycles := func(graph *Graph) bool {
		decycle(graph)
		err := graph.CheckErrors()
		if err != nil {
			t.Errorf("invalid %v:\n%v", err, graph.EdgeTableString())
			return false
		}
		if graph.IsCyclic() {
			t.Errorf("cyclic:\n%v", graph.EdgeTableString())
			return false
		}
		return true
	}

	if err := quick.Check(noCycles, nil); err != nil {
		t.Error(err)
	}
}
