package hier

import "testing"

// TestAddVirtualsParallelWeight checks that every chain of parallel long
// edges keeps the weight of its edge.
func TestAddVirtualsParallelWeight(t *testing.T) {
	graph := NewGraph()
	a, b, c := graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(a, b)
	graph.AddEdge(b, c)
	graph.AddWeightedEdge(a, c, 5)
	graph.AddWeightedEdge(a, c, 5)

	Rank(graph)
	AddVirtuals(graph)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if (src.Virtual || dst.Virtual) && graph.Weight(src, dst) != 5 {
				t.Errorf("edge %v->%v has weight %v, want 5", src, dst, graph.Weight(src, dst))
			}
		}
	}
}
