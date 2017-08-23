package layout

import "testing"

func TestIsCyclic(t *testing.T) {
	for _, testgraph := range TestGraphs {
		graph := testgraph.Make()
		if graph.IsCyclic() != testgraph.Cyclic {
			t.Errorf("%v: expected IsCyclic = %v", testgraph.Name, testgraph.Cyclic)
		}
		if err := graph.CheckErrors(); err != nil {
			t.Errorf("%v: %v", testgraph.Name, err)
		}
	}
}
