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

// TestClusterFitsLabel checks that a cluster is at least as wide as its
// label across the top, and that its members stay inside it.
func TestClusterFitsLabel(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Clusters = []*layout.Cluster{{ID: "c", Label: "Extremely long cluster label spanning many characters", Nodes: []*layout.Node{graph.Node("a")}}}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	cluster, a := graph.Clusters[0], graph.Node("a")
	// most characters are about half an em wide, spaces and narrow
	// letters less
	if width := cluster.BottomRight.X - cluster.TopLeft.X; width < Length(len(cluster.Label))*graph.FontSize*0.45 {
		t.Errorf("cluster is %v wide, too narrow for its label", width)
	}
	if a.Left() < cluster.TopLeft.X || a.Right() > cluster.BottomRight.X {
		t.Errorf("a at %v-%v outside the cluster at %v-%v", a.Left(), a.Right(), cluster.TopLeft.X, cluster.BottomRight.X)
	}
}

type Length = layout.Length
