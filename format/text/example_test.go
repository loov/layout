package text_test

import (
	"log"
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/text"
)

// Options.ForText lays the graph out for a character grid; without it the
// drawing works but edges may come out as staircases.
func ExampleWrite() {
	graph := layout.NewDigraph()
	graph.Edge("a", "b")
	graph.Edge("a", "c")
	graph.Edge("b", "d")
	graph.Edge("c", "d")
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		log.Fatal(err)
	}
	if err := text.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	//      ╭───╮
	//      │ a │
	//      ╰─┬┬╯
	//     ╭──╯╰──╮
	//     ▼      ▼
	//   ╭───╮  ╭───╮
	//   │ b │  │ c │
	//   ╰─┬─╯  ╰─┬─╯
	//     ╰──╮╭──╯
	//        ▼▼
	//      ╭───╮
	//      │ d │
	//      ╰───╯
}

// WriteColor adds the node, edge and cluster colors as ANSI escape codes
// for a terminal.
func ExampleWriteColor() {
	graph := layout.NewDigraph()
	ok := graph.Edge("build", "deploy")
	ok.LineColor = layout.RGB{G: 0x80}
	graph.Node("deploy").FillColor = layout.RGB{R: 0x98, G: 0xFB, B: 0x98}
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		log.Fatal(err)
	}
	opts := text.Options{Palette: text.TrueColor}
	if err := text.WriteColor(os.Stdout, l, opts); err != nil {
		log.Fatal(err)
	}
}
