package hier

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
					beforeCount := graph.CountUndirectedLinks()

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

					afterCount := graph.CountUndirectedLinks()
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

func BenchmarkDecycle(b *testing.B) {
	for _, decyclerCase := range decyclerCases {
		b.Run(decyclerCase.name, func(b *testing.B) {
			for _, size := range BenchmarkGraphSizes {
				b.Run(size.Name, func(b *testing.B) {
					graph := GenerateRegularGraph(size.Nodes, size.Connections)

					for i := 0; i < b.N; i++ {
						decycle := NewDecycle(graph)
						decycle.Recurse = decyclerCase.recurse
						decycle.Reorder = decyclerCase.reorder
						decycle.SkipUpdate = true
						decycle.Run()
					}
				})
			}
		})
	}
}
