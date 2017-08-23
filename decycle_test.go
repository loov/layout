package layout

import (
	"testing"
	"testing/quick"
)

var decyclerCases = []struct {
	name             string
	recurse, reorder bool
}{
	{"Basic", false, false},
	{"Recurse", true, false},
	{"Reorder", false, true},
	{"RecurseReorder", true, true},
}

func TestDecycle(t *testing.T) {
	for _, decyclerCase := range decyclerCases {
		t.Run(decyclerCase.name, func(t *testing.T) {
			for _, testgraph := range TestGraphs {
				t.Run(testgraph.Name, func(t *testing.T) {
					graph := testgraph.Make()
					beforeCount := countLinks(graph)

					decycle := NewDecycle(graph)
					decycle.Recurse = decyclerCase.recurse
					decycle.Reorder = decyclerCase.reorder
					decycle.Run()

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
						t.Log("edge table: \n" + graph.EdgeMatrixString())
					}
				})
			}
		})
	}
}

func countLinks(graph *Graph) int {
	edges := NewEdgeSet()
	for _, node := range graph.Nodes {
		for _, out := range node.Out {
			if out != node {
				edges.Include(node, out)
			}
		}
	}
	return len(edges)
}

func TestDecycleRandom(t *testing.T) {
	for _, decyclerCase := range decyclerCases {
		t.Run(decyclerCase.name, func(t *testing.T) {
			err := quick.Check(func(graph *Graph) bool {
				decycle := NewDecycle(graph)
				decycle.Recurse = decyclerCase.recurse
				decycle.Reorder = decyclerCase.reorder
				decycle.Run()

				err := graph.CheckErrors()
				if err != nil {
					t.Errorf("invalid %v:\n%v", err, graph.EdgeMatrixString())
					return false
				}
				if graph.IsCyclic() {
					t.Errorf("cyclic:\n%v", graph.EdgeMatrixString())
					return false
				}
				return true
			}, nil)

			if err != nil {
				t.Error(err)
			}
		})
	}
}
