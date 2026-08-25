package layout_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/loov/layout"
)

// FuzzHierarchical builds a graph from random bytes with edges, self loops,
// same-rank groups, labels, ports, min/max rank and nested clusters, and
// checks that layouting neither panics nor produces non-finite positions.
func FuzzHierarchical(f *testing.F) {
	f.Add([]byte{4, 0, 1, 1, 2, 2, 3, 3, 0})
	f.Add([]byte{6, 0, 1, 1, 2, 2, 3, 0x40 | 3, 4, 0x80 | 1, 5, 0xC0 | 2, 5, 0x21, 1, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 1 {
			return
		}
		n := int(data[0])%12 + 1
		data = data[1:]
		graph := layout.NewDigraph()
		nodes := make([]*layout.Node, n)
		for i := range nodes {
			nodes[i] = graph.Node(fmt.Sprint("n", i))
		}
		ports := []layout.Compass{layout.CompassAuto, layout.North, layout.South, layout.East, layout.West}
		var clusters []*layout.Cluster
		for i := 0; i+1 < len(data); i += 2 {
			a, b := data[i], data[i+1]
			from, to := nodes[int(a&15)%n], nodes[int(b&15)%n]
			switch a >> 6 {
			case 0, 1:
				edge := graph.Edge(from.ID, to.ID)
				if a&0x20 != 0 {
					edge.Label = "label"
				}
				edge.FromPort = ports[int(b>>4)%len(ports)]
				edge.ToPort = ports[int(b>>5)%len(ports)]
			case 2:
				switch b >> 4 {
				case 0:
					graph.SameRank = append(graph.SameRank, []*layout.Node{from, to})
				case 1:
					graph.MinRank = append(graph.MinRank, from)
				case 2:
					graph.MaxRank = append(graph.MaxRank, to)
				}
			case 3:
				// cluster of nodes from..to, nested into the last cluster
				// containing all of them
				lo, hi := min(from.ID, to.ID), max(from.ID, to.ID)
				cluster := &layout.Cluster{ID: fmt.Sprint("cluster", len(clusters)), Label: "c"}
				for _, node := range nodes {
					if node.ID >= lo && node.ID <= hi {
						cluster.Nodes = append(cluster.Nodes, node)
					}
				}
				// clusters must nest or be disjoint
				valid := true
				for _, other := range clusters {
					switch {
					case contains(other, cluster):
						cluster.Parent = other
					case contains(cluster, other):
						if other.Parent == cluster.Parent {
							other.Parent = cluster
						}
					case overlaps(other, cluster):
						valid = false
					}
				}
				if valid {
					clusters = append(clusters, cluster)
				}
			}
		}
		graph.Clusters = clusters
		if err := layout.Hierarchical(graph); err != nil {
			return // validation errors are fine
		}
		for _, node := range graph.Nodes {
			if !finite(node.Center) {
				t.Fatalf("node %v at %v", node.ID, node.Center)
			}
		}
		for _, edge := range graph.Edges {
			for _, p := range edge.Path {
				if !finite(p) {
					t.Fatalf("edge %v -> %v path %v", edge.From.ID, edge.To.ID, edge.Path)
				}
			}
		}
		for _, cluster := range clusters {
			if !finite(cluster.TopLeft) || !finite(cluster.BottomRight) {
				t.Fatalf("cluster %v box %v %v", cluster.ID, cluster.TopLeft, cluster.BottomRight)
			}
		}
	})
}

func contains(outer, inner *layout.Cluster) bool {
	for _, node := range inner.Nodes {
		found := false
		for _, o := range outer.Nodes {
			found = found || o == node
		}
		if !found {
			return false
		}
	}
	return true
}

func overlaps(a, b *layout.Cluster) bool {
	for _, x := range a.Nodes {
		for _, y := range b.Nodes {
			if x == y {
				return true
			}
		}
	}
	return false
}

func finite(v layout.Vector) bool {
	return !math.IsNaN(float64(v.X)) && !math.IsInf(float64(v.X), 0) &&
		!math.IsNaN(float64(v.Y)) && !math.IsInf(float64(v.Y), 0)
}
