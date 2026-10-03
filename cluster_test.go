package layout_test

import (
	"slices"
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

// checkSiblingClusters checks that clusters without a common parent don't
// overlap and that no node lies inside a cluster it isn't a member of.
func checkSiblingClusters(t *testing.T, graph *layout.Graph) {
	t.Helper()
	for i, a := range graph.Clusters {
		for _, b := range graph.Clusters[i+1:] {
			if a.Parent != b.Parent {
				continue
			}
			if a.TopLeft.X < b.BottomRight.X && b.TopLeft.X < a.BottomRight.X && a.TopLeft.Y < b.BottomRight.Y && b.TopLeft.Y < a.BottomRight.Y {
				t.Errorf("clusters %s %v-%v and %s %v-%v overlap", a.ID, a.TopLeft, a.BottomRight, b.ID, b.TopLeft, b.BottomRight)
			}
		}
		for _, node := range graph.Nodes {
			if slices.Contains(a.Nodes, node) {
				continue
			}
			if c := node.Center; c.X > a.TopLeft.X && c.X < a.BottomRight.X && c.Y > a.TopLeft.Y && c.Y < a.BottomRight.Y {
				t.Errorf("%s at %v lies inside cluster %s %v-%v", node.ID, c, a.ID, a.TopLeft, a.BottomRight)
			}
		}
	}
}

// TestClusterSiblingsDontInterleave checks that two clusters whose nodes
// have the same mean position in a rank stay apart instead of mixing.
func TestClusterSiblingsDontInterleave(t *testing.T) {
	graph := layout.NewDigraph()
	for _, e := range [][2]string{{"n3", "n4"}, {"n1", "n5"}, {"n1", "n4"}, {"n2", "n4"}, {"n0", "n2"}, {"n5", "n4"}, {"n0", "n4"}} {
		graph.Edge(e[0], e[1])
	}
	graph.Clusters = []*layout.Cluster{
		{ID: "A", Nodes: []*layout.Node{graph.Node("n4"), graph.Node("n3")}},
		{ID: "B", Nodes: []*layout.Node{graph.Node("n1"), graph.Node("n2")}},
	}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	checkSiblingClusters(t, graph)
}

// TestClusterNeighborsAlign checks that two clusters side by side, each
// pushing the other while lining up its borders, settle at a sane width.
func TestClusterNeighborsAlign(t *testing.T) {
	graph := layout.NewDigraph()
	for _, e := range [][2]string{{"n6", "n0"}, {"n1", "n2"}, {"n7", "n5"}, {"n6", "n1"}, {"n6", "n5"}, {"n5", "n1"}, {"n1", "n5"}, {"n4", "n1"}, {"n7", "n3"}, {"n7", "n2"}, {"n3", "n2"}, {"n1", "n0"}, {"n7", "n0"}, {"n7", "n0"}, {"n4", "n2"}} {
		graph.AddEdge(layout.NewEdge(graph.Node(e[0]), graph.Node(e[1])))
	}
	graph.Clusters = []*layout.Cluster{
		{ID: "A", Nodes: []*layout.Node{graph.Node("n0"), graph.Node("n2"), graph.Node("n4")}},
		{ID: "B", Nodes: []*layout.Node{graph.Node("n7"), graph.Node("n5")}},
	}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	checkSiblingClusters(t, graph)
	// every node side by side, with room to spare
	var total Length
	for _, node := range graph.Nodes {
		total += 2 * node.Radius.X
	}
	for _, cluster := range graph.Clusters {
		if width := cluster.BottomRight.X - cluster.TopLeft.X; width > total {
			t.Errorf("cluster %s is %v wide, want at most %v", cluster.ID, width, total)
		}
	}
}

type Length = layout.Length
