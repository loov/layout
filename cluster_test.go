package layout_test

import (
	"testing"

	"github.com/loov/layout"
)

// TestClusterExcludesPassingEdges checks that an edge between nodes
// outside a cluster runs beside the cluster, not through its box, when it
// passes the cluster's rank right at the cluster's position.
func TestClusterExcludesPassingEdges(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Edge("checkout", "build")
	graph.Edge("checkout", "lint")
	graph.Edge("build", "unit")
	graph.Edge("build", "integration")
	passing := graph.Edge("lint", "review")
	graph.Edge("unit", "review")
	graph.Edge("integration", "review").Label = "slow"
	graph.Edge("review", "deploy").Label = "approve"
	graph.Clusters = []*layout.Cluster{{ID: "test", Nodes: []*layout.Node{graph.Node("unit"), graph.Node("integration")}}}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	cluster := graph.Clusters[0]
	for _, p := range passing.Path {
		if p.X > cluster.TopLeft.X && p.X < cluster.BottomRight.X && p.Y > cluster.TopLeft.Y && p.Y < cluster.BottomRight.Y {
			t.Errorf("lint -> review passes through the cluster at %v, cluster %v-%v", p, cluster.TopLeft, cluster.BottomRight)
		}
	}
}
