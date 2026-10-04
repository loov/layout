package layout_test

import (
	"fmt"
	"log"
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
	"github.com/loov/layout/format/text"
)

// The usual flow: build a graph, lay it out and write it in some format.
func Example() {
	graph := layout.NewDigraph()
	graph.Edge("A", "B")
	graph.Edge("A", "C")
	graph.Edge("B", "D")
	graph.Edge("C", "D")

	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	if err := svg.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
}

// Hierarchical returns where each node and edge goes, which is all a
// custom renderer needs.
func ExampleHierarchical() {
	graph := layout.NewDigraph()
	graph.Edge("A", "B")
	graph.Edge("A", "C")

	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	for i, node := range graph.Nodes {
		fmt.Printf("%s at %v,%v\n", node.ID, l.Nodes[i].Center.X, l.Nodes[i].Center.Y)
	}
	for i, edge := range graph.Edges {
		fmt.Printf("%v through %v\n", edge, l.Edges[i].Path)
	}
	// Output:
	// A at 64,32
	// B at 32,96
	// C at 96,96
	// A->B through [{56.698177 46.23669} {38.64111 81.44337}]
	// A->C through [{71.30183 46.23669} {89.35889 81.44337}]
}

// Fast spends fewer sweeps on reducing crossings, for large graphs;
// Quality spends more. Align packs the nodes to one side.
func ExampleOptions() {
	graph := layout.NewDigraph()
	graph.Edge("root", "a")
	graph.Edge("root", "b")
	graph.Edge("a", "leaf")

	opts := layout.Fast
	opts.Align = layout.AlignLeft
	opts.ForText = true
	l, err := layout.Hierarchical(graph, opts)
	if err != nil {
		log.Fatal(err)
	}
	if err := text.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	//   ╭──────╮
	//   │ root │
	//   ╰───┬─┬╯
	//       │ │
	//       │ ╰─────╮
	//       │       │
	//       ▼       ▼
	//     ╭───╮   ╭───╮
	//     │ a │   │ b │
	//     ╰─┬─╯   ╰───╯
	//       │
	//       ▼
	//   ╭──────╮
	//   │ leaf │
	//   ╰──────╯
}

// Force suits graphs without a direction, such as a cycle.
func ExampleForce() {
	graph := layout.NewGraph()
	graph.Edge("a", "b")
	graph.Edge("b", "c")
	graph.Edge("c", "a")

	l, err := layout.Force(graph, layout.ForceOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for i, edge := range graph.Edges {
		fmt.Printf("%v: %d points\n", edge, len(l.Edges[i].Path))
	}
	// Output:
	// a->b: 2 points
	// b->c: 2 points
	// c->a: 2 points
}

// Options.ForText lays the graph out for drawing on a character grid.
func Example_text() {
	graph := layout.NewDigraph()
	graph.Edge("fetch", "parse")
	graph.Edge("parse", "render").Label = "ok"
	graph.Edge("parse", "report").Label = "error"

	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		log.Fatal(err)
	}
	if err := text.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	//            ╭───────╮
	//            │ fetch │
	//            ╰───┬───╯
	//                │
	//                ▼
	//            ╭───────╮
	//            │ parse │
	//            ╰──┬──┬─╯
	//               │  │
	//        ╭──────╯  ╰──────╮
	//        │                │
	//        │ ok             │ error
	//        │                │
	//        ▼                ▼
	//   ╭────────╮       ╭────────╮
	//   │ render │       │ report │
	//   ╰────────╯       ╰────────╯
}

// RankDir turns the layout sideways.
func Example_rankDir() {
	graph := layout.NewDigraph()
	graph.RankDir = layout.LeftToRight
	graph.Edge("idle", "running")
	graph.Edge("running", "done")

	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		log.Fatal(err)
	}
	if err := text.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	//   ╭──────╮   ╭─────────╮   ╭──────╮
	//   │ idle ├──▶│ running ├──▶│ done │
	//   ╰──────╯   ╰─────────╯   ╰──────╯
}

// Clusters draw a box around a group of nodes, which the layout keeps
// together.
func Example_clusters() {
	graph := layout.NewDigraph()
	graph.Edge("client", "api")
	graph.Edge("api", "db")
	graph.Edge("api", "cache")
	graph.Clusters = []*layout.Cluster{{
		ID:    "backend",
		Label: "backend",
		Nodes: []*layout.Node{graph.Node("db"), graph.Node("cache")},
	}}

	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		log.Fatal(err)
	}
	if err := text.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	//               ╭────────╮
	//               │ client │
	//               ╰───┬────╯
	//                   │
	//                   ▼
	//                ╭─────╮
	//                │ api │
	//                ╰─┬─┬─╯
	//                  │ │
	//            ╭─────╯ ╰──────╮
	//            │              │
	// ┌ backend ┈╂┈┈┈┈┈┈┈┈┈┈┈┈┈┈╂┈┈┈┈┈┈┐
	// ┊          ▼              ▼      ┊
	// ┊       ╭────╮        ╭───────╮  ┊
	// ┊       │ db │        │ cache │  ┊
	// ┊       ╰────╯        ╰───────╯  ┊
	// ┊                                ┊
	// └┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┘
}

// A Record node draws its label as a table of fields, using the
// Graphviz record syntax: | separates fields and braces flip between
// rows and columns.
func Example_records() {
	graph := layout.NewDigraph()
	user := graph.Node("user")
	user.Shape = layout.Record
	user.Label = "User|{id|name}"
	graph.Edge("user", "session")

	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		log.Fatal(err)
	}
	if err := text.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	//   ┌──────────────┐
	//   │      │  id   │
	//   │ User │───────│
	//   │      │ name  │
	//   └──────┬───────┘
	//          │
	//          ▼
	//     ╭─────────╮
	//     │ session │
	//     ╰─────────╯
}

// SameRank, MinRank and MaxRank constrain which rank nodes go on.
func Example_ranks() {
	graph := layout.NewDigraph()
	graph.Edge("a", "b")
	graph.Edge("b", "c")
	graph.Edge("x", "y")
	graph.SameRank = [][]*layout.Node{{graph.Node("b"), graph.Node("x")}}
	graph.MaxRank = []*layout.Node{graph.Node("y")}

	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	for _, id := range []string{"b", "x", "c", "y"} {
		fmt.Printf("%s at y=%v\n", id, l.Node(graph.Node(id)).Center.Y)
	}
	// Output:
	// b at y=96
	// x at y=96
	// c at y=160
	// y at y=160
}

// When every node has a Pos, layout keeps the positions and only routes
// the edges, like dot -n.
func Example_pinned() {
	graph := layout.NewDigraph()
	graph.Node("a").Pos = &layout.Vector{X: 0, Y: 0}
	graph.Node("b").Pos = &layout.Vector{X: 200, Y: 100}
	graph.Edge("a", "b")

	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(l.Node(graph.Node("b")).Center)
	fmt.Println(len(l.Edges[0].Path) > 0)
	// Output:
	// {200 100}
	// true
}

// MeasureText replaces the built-in text width estimate, for example
// with measurements from the font the output will use. It returns the
// width of one line.
func Example_measureText() {
	graph := layout.NewDigraph()
	graph.MeasureText = func(line, fontName string, fontSize layout.Length) layout.Length {
		// a monospace font with characters 0.6 em wide
		return layout.Length(len(line)) * fontSize * 0.6
	}
	graph.Edge("short", "a much longer label")

	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	for i, node := range graph.Nodes {
		fmt.Printf("%q is %v wide\n", node.ID, l.Nodes[i].Size.X)
	}
	// Output:
	// "short" is 56 wide
	// "a much longer label" is 173.6 wide
}

// Nodes, edges and clusters carry their own styling.
func Example_styling() {
	graph := layout.NewDigraph()
	start := graph.Node("start")
	start.Shape = layout.PointShape
	ok := graph.Node("ok")
	ok.Shape = layout.Box
	ok.FillColor = layout.RGB{R: 0x98, G: 0xFB, B: 0x98}
	ok.Peripheries = 2

	graph.Edge("start", "check")
	pass := graph.Edge("check", "ok")
	pass.Label = "pass"
	pass.LineColor = layout.RGB{G: 0x80}
	fail := graph.Edge("check", "retry")
	fail.LineStyle = layout.Dashed
	fail.ArrowHead = layout.ArrowVee
	fail.FromPort = layout.East

	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	if err := svg.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
}

// Diagnose counts layout defects, such as edge crossings, so that tests
// can check layout quality without looking at the drawing.
func ExampleDiagnose() {
	graph := layout.NewDigraph()
	graph.Edge("a", "d")
	graph.Edge("b", "c")
	graph.Edge("a", "c")
	graph.Edge("b", "d")

	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	diag := layout.Diagnose(l)
	fmt.Println("node overlaps:", diag.NodeOverlaps)
	fmt.Println("edge crossings:", diag.EdgeCrossings)
	// Output:
	// node overlaps: 0
	// edge crossings: 1
}

func ExampleColorByName() {
	color, ok := layout.ColorByName("forestgreen")
	fmt.Println(ok)
	fmt.Println(color.RGBA8())
	// Output:
	// true
	// 34 139 34 255
}

// Edge finds or creates the edge and both of its nodes.
func ExampleGraph_Edge() {
	graph := layout.NewDigraph()
	first := graph.Edge("a", "b")
	again := graph.Edge("a", "b")

	fmt.Println(first == again)
	fmt.Println(len(graph.Nodes), len(graph.Edges))
	// Output:
	// true
	// 2 1
}
