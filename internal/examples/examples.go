// Package examples holds the graphs that the tests draw and compare to
// the files in testdata, shared by the tests of the packages.
package examples

import (
	"strings"

	"github.com/loov/layout"
)

var Graphs = map[string]func() *layout.Graph{
	// flat_labels has labeled edges between nodes of a rank, next to each
	// other and arcing over one between, see FlatLabels
	"flat_labels": FlatLabels,
	"flat_labels_lr": func() *layout.Graph {
		graph := FlatLabels()
		graph.RankDir = layout.LeftToRight
		return graph
	},
	// narrow_labels has edge labels of narrow letters, which take as many
	// cells as any other letters in text
	"narrow_labels": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("a", "b").Label = "iiiiiiiiiiiiiiii"
		graph.Edge("a", "c").Label = "llllllllllllllll"
		graph.Edge("a", "d").Label = "1.1.1.1"
		graph.Edge("b", "e")
		graph.Edge("c", "e")
		graph.Edge("d", "e")
		return graph
	},
	// circles_lr makes every node a circle through the graph's default
	// shape, sideways, where edge ends packed along nodes make them tall
	"circles_lr": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		graph.Shape = layout.Circle
		for _, to := range []string{"b", "c", "d", "e"} {
			graph.Edge("a", to)
			graph.Edge(to, "f")
		}
		return graph
	},
	// cluster_lr has cluster labels longer than their members, sideways,
	// where the labels run along the ranks
	"cluster_lr": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		graph.Edge("a", "b")
		graph.Edge("a", "c")
		graph.Edge("c", "d")
		graph.Clusters = []*layout.Cluster{
			{ID: "x", Label: "a cluster label that is long", Nodes: []*layout.Node{graph.Node("b")}},
			{ID: "y", Label: "second one", Nodes: []*layout.Node{graph.Node("c"), graph.Node("d")}},
		}
		return graph
	},
	// readme is the example in README.md; keep the two in sync
	"readme": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("checkout", "build")
		graph.Edge("checkout", "lint")
		graph.Edge("build", "unit")
		graph.Edge("build", "integration")
		graph.Edge("lint", "review").LineStyle = layout.Dashed
		graph.Edge("unit", "review")
		graph.Edge("integration", "review").Label = "slow"

		approve := graph.Edge("review", "deploy")
		approve.Label = "approve"
		approve.LineColor = layout.RGB{G: 0x80}
		graph.Node("review").Shape = layout.Box
		graph.Node("deploy").FillColor = layout.RGB{R: 0x98, G: 0xFB, B: 0x98}

		graph.Clusters = []*layout.Cluster{{
			ID: "test", Label: "test",
			Nodes:     []*layout.Node{graph.Node("unit"), graph.Node("integration")},
			LineColor: layout.RGB{B: 0xFF},
		}}
		return graph
	},
	// regex is a DFA for -?[0-9]+(\.[0-9]+)?, drawn left to right like
	// automata usually are: a start arrow, edge labels on every edge,
	// self-loops and accepting states
	"regex": func() *layout.Graph {
		graph := regexDFA()
		graph.RankDir = layout.LeftToRight
		return graph
	},
	"regex_tb": regexDFA,
	"minimal": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		return graph
	},
	"basic": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Node("A")
		graph.Node("B")
		graph.Node("C")
		graph.Node("D")
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		graph.Edge("D", "A")
		return graph
	},
	"weighted": func() *layout.Graph {
		// K2,2 must have one crossing; the heavy P->C edge should stay straight
		graph := layout.NewDigraph()
		graph.Edge("P", "B")
		graph.Edge("P", "C").Weight = 10
		graph.Edge("Q", "B")
		graph.Edge("Q", "C")
		return graph
	},
	"flat": func() *layout.Graph {
		// A, B, C on one rank with A->B adjacent and A->C arcing over B
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		graph.Edge("R", "A")
		graph.Edge("R", "B")
		graph.Edge("R", "C")
		graph.SameRank = append(graph.SameRank, []*layout.Node{graph.Node("A"), graph.Node("B"), graph.Node("C")})
		return graph
	},
	"loop": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "A")
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "B")
		graph.Edge("C", "D")
		return graph
	},
	"multi": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.AddEdge(layout.NewEdge(graph.Node("A"), graph.Node("B")))
		graph.AddEdge(layout.NewEdge(graph.Node("A"), graph.Node("B")))
		graph.AddEdge(layout.NewEdge(graph.Node("A"), graph.Node("B")))
		graph.Edge("B", "C")
		graph.Edge("C", "B")
		graph.Edge("A", "D")
		graph.Edge("D", "C")
		return graph
	},
	"rankdir": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		a := graph.Node("A")
		a.Shape = layout.Box
		a.Label = "Left\nto\nright"
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("B", "D")
		graph.Edge("C", "D")
		graph.Edge("D", "A")
		graph.Edge("D", "D")
		return graph
	},
	"minmax": func() *layout.Graph {
		// X is pinned to the top and Y to the bottom despite their edges
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("B", "C")
		graph.Edge("C", "D")
		graph.Edge("B", "X")
		graph.Edge("Y", "C")
		graph.MinRank = []*layout.Node{graph.Node("X")}
		graph.MaxRank = []*layout.Node{graph.Node("Y")}
		return graph
	},
	"components": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		graph.Edge("X", "Y")
		graph.Edge("Y", "Z")
		graph.Edge("Z", "X")
		graph.Node("Lonely")
		return graph
	},
	"labels": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("A", "B").Label = "yes"
		graph.Edge("A", "C").Label = "no"
		graph.Edge("B", "D")
		graph.Edge("C", "D").Label = "long label here"
		graph.Edge("D", "A").Label = "back"
		return graph
	},
	"arrows": func() *layout.Graph {
		graph := layout.NewDigraph()
		both := graph.Edge("A", "B")
		both.ArrowHead, both.ArrowTail = layout.ArrowNormal, layout.ArrowNormal
		graph.Edge("A", "C").ArrowHead = layout.ArrowDot
		graph.Edge("A", "D").ArrowHead = layout.ArrowODot
		graph.Edge("B", "E").ArrowHead = layout.ArrowVee
		graph.Edge("C", "E").ArrowHead = layout.ArrowNone
		ports := graph.Edge("D", "E")
		ports.FromPort, ports.ToPort = layout.West, layout.East
		return graph
	},
	"cluster": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Edge("start", "a0")
		graph.Edge("start", "b0")
		graph.Edge("a0", "a1")
		graph.Edge("a1", "a2")
		graph.Edge("b0", "b1")
		graph.Edge("a1", "b1")
		graph.Edge("a2", "end")
		graph.Edge("b1", "end")
		graph.Edge("start", "end")
		graph.Clusters = []*layout.Cluster{
			{ID: "cluster_a", Label: "A side", Nodes: []*layout.Node{graph.Node("a0"), graph.Node("a1"), graph.Node("a2")}, FillColor: layout.RGB{R: 0xEE, G: 0xEE, B: 0xFF}},
			{ID: "cluster_b", Label: "B side", Nodes: []*layout.Node{graph.Node("b0"), graph.Node("b1")}, LineColor: layout.RGB{R: 0, G: 0, B: 0xFF}},
		}
		return graph
	},
	// cluster_labels has a long edge label into a cluster that has no room
	// beside its line, where the other side of the line is across the
	// cluster box
	"cluster_labels": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.Shape = layout.Box
		graph.Node("switch").Label = "switch\nport-a  port-b  port-c  port-d  port-e  port-f  port-g  port-h  port-i  port-j"
		graph.Edge("switch", "hub").Label = "uplink → hub-in"
		graph.Edge("hub", "serial").Label = "usb → serial"
		graph.Edge("hub", "router").Label = "console via extension cable"
		graph.Edge("uart", "hub").Label = "tx → hub-serial"
		graph.Edge("serial", "uart").Label = "null-modem cable with a rather long name here\nconsole, 115200\n1 → tty1"
		graph.Clusters = []*layout.Cluster{
			{ID: "server", Label: "server", Nodes: []*layout.Node{graph.Node("gpu"), graph.Node("uart")}},
		}
		return graph
	},
	"record": func() *layout.Graph {
		graph := layout.NewDigraph()
		a := graph.Node("A")
		a.Shape = layout.Record
		a.Label = "<f0> left|<f1> middle|{top|bottom}"
		b := graph.Node("B")
		b.Shape = layout.Record
		b.Label = "{name|type|value}"
		graph.Edge("A", "B")
		graph.Edge("A", "C")
		return graph
	},
	"complex": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RowPadding = 30 * layout.Point

		a := graph.Node("A")
		a.Shape = layout.Box
		a.Label = "Lorem\nIpsum\nDolorem"
		a.FillColor = layout.RGB{R: 0xFF, G: 0xA0, B: 0x20}

		b := graph.Node("B")
		b.Shape = layout.Ellipse
		b.Label = "Ignitus"
		b.FillColor = layout.HSL{H: 0, S: 0.7, L: 0.7}

		c := graph.Node("C")
		c.Shape = layout.Square
		c.FontSize = 12 * layout.Point
		c.FontColor = layout.RGB{R: 0x20, G: 0x20, B: 0x20}

		graph.Node("D")

		ab := graph.Edge("A", "B")
		ab.LineWidth = 4 * layout.Point
		ac := graph.Edge("A", "C")
		ac.LineWidth = 4 * layout.Point
		if col, ok := layout.ColorByName("blue"); ok {
			ac.LineColor = col
		}
		bd := graph.Edge("B", "D")
		bd.LineColor = layout.RGB{R: 0xA0, G: 0xFF, B: 0xA0}
		graph.Edge("C", "D")
		graph.Edge("D", "A")
		return graph
	},
	// subnets is an IPv6 address plan in the documentation prefix, drawn
	// left to right, with labels of two lines stacked down the ranks
	"subnets": func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		for _, n := range [][2]string{
			{"ula", "2001:db8:abcd::/48\nsite"},
			{"s00", "2001:db8:abcd::/56\nsite 00 (network)"},
			{"s01", "2001:db8:abcd:0100::/56\nsite 01 (north)"},
			{"s02", "2001:db8:abcd:0200::/56\nsite 02 (west)"},
			{"s03", "2001:db8:abcd:0300::/56\nsite 03 (east)"},
			{"remote", "2001:db8:abcd:fb00::/56\nremote access"},
			{"circuit", "2001:db8:abcd:fc00::/56\ncircuit /127s"},
			{"srv6", "2001:db8:abcd:fd00::/56\nSRv6 locators"},
			{"carrier", "2001:db8:abcd:fe00::/56\ncarrier /127s"},
			{"lab", "2001:db8:abcd:ff00::/56\nlab"},
			{"loopback", "2001:db8:abcd::/64\nrouter loopbacks"},
			{"anycast", "2001:db8:abcd:1::/64\nanycast"},
			{"dns", "2001:db8:abcd:1::53\ndns"},
			{"ntp", "2001:db8:abcd:1::123\nntp"},
			{"mgmt", "2001:db8:abcd:100::/64\nmgmt0, VLAN 0 (mgmt)"},
			{"guest", "2001:db8:abcd:109::/64\nguest0, VLAN 9 (guest)"},
			{"lan", "2001:db8:abcd:110::/64\nlan0, VLAN 10 (lan)"},
			{"dev", "2001:db8:abcd:120::/64\ndev0, VLAN 20 (dev)"},
			{"iot", "2001:db8:abcd:166::/64\niot0, VLAN 66 (iot)"},
		} {
			node := graph.Node(n[0])
			node.Label = n[1]
			// prefixes split further are boxes, networks are rounded
			if strings.Contains(n[1], "/48") || strings.Contains(n[1], "/56") {
				node.Shape = layout.Box
			}
		}
		for _, e := range []struct {
			from string
			to   []string
		}{
			{"ula", []string{"s00", "s01", "s02", "s03", "remote", "circuit", "srv6", "carrier", "lab"}},
			{"s00", []string{"loopback", "anycast"}},
			{"anycast", []string{"dns", "ntp"}},
			{"s01", []string{"mgmt", "guest", "lan", "dev", "iot"}},
		} {
			for _, to := range e.to {
				graph.Edge(e.from, to)
			}
		}
		return graph
	},
}

// Options are the layout options of examples that don't use the
// defaults
var Options = map[string]layout.Options{
	// automata read best with the path from the start along one line
	"regex":    {Align: layout.AlignLeft},
	"regex_tb": {Align: layout.AlignLeft},
}

// regexDFA returns the automaton of the regex examples
func regexDFA() *layout.Graph {
	graph := layout.NewDigraph()
	graph.Node("start").Shape = layout.PointShape
	for _, id := range []string{"s0", "s1", "s2", "s3", "s4"} {
		graph.Node(id).Shape = layout.Circle
	}
	for _, id := range []string{"s2", "s4"} {
		accept := graph.Node(id)
		accept.FillColor = layout.RGB{R: 0x98, G: 0xFB, B: 0x98}
		accept.Peripheries = 2
	}
	graph.Edge("start", "s0")
	graph.Edge("s0", "s1").Label = "-"
	graph.Edge("s0", "s2").Label = "0-9"
	graph.Edge("s1", "s2").Label = "0-9"
	graph.Edge("s2", "s2").Label = "0-9"
	graph.Edge("s2", "s3").Label = "."
	graph.Edge("s3", "s4").Label = "0-9"
	graph.Edge("s4", "s4").Label = "0-9"
	return graph
}

// merged edge variants of the examples with edges that fan out or in,
// see layout.Graph.MergeEdges
func init() {
	for _, name := range []string{"minimal", "readme", "cluster", "flat", "rankdir", "subnets"} {
		build := Graphs[name]
		Graphs[name+"_merged"] = func() *layout.Graph {
			graph := build()
			graph.MergeEdges = true
			return graph
		}
	}
}

// FlatLabels returns a graph with labeled edges between nodes of a rank:
// next to each other, and arcing over one between them
func FlatLabels() *layout.Graph {
	graph := layout.NewDigraph()
	graph.Edge("a", "b").Label = "flat label"
	graph.Edge("b", "c").Label = "x"
	graph.Edge("a", "c").Label = "arc"
	graph.Edge("a", "d")
	graph.Edge("a", "e").Label = "down"
	graph.SameRank = [][]*layout.Node{{graph.Node("a"), graph.Node("b"), graph.Node("c")}}
	return graph
}
