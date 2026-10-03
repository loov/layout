package dot_test

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/text"
)

// A dot file can be laid out and written in another format.
func Example() {
	graphs, err := dot.ParseString(`digraph {
		rankdir=LR
		a -> b -> c
		a [shape=box]
	}`)
	if err != nil {
		log.Fatal(err)
	}
	l, err := layout.Hierarchical(graphs[0], layout.Options{ForText: true})
	if err != nil {
		log.Fatal(err)
	}
	if err := text.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	//   ┌───┐   ╭────╮   ╭───╮
	//   │ a ├──▶│ b  ├──▶│ c │
	//   └───┘   ╰────╯   ╰───╯
}

func ExampleParse() {
	input := strings.NewReader(`graph G { a -- b; b -- c }`)
	graphs, err := dot.Parse(input)
	if err != nil {
		log.Fatal(err)
	}
	for _, graph := range graphs {
		fmt.Println(graph.ID, graph.Directed, len(graph.Nodes), len(graph.Edges))
	}
	// Output:
	// G false 3 2
}

// ParseString returns every graph in the input; attributes are applied
// to the nodes and edges.
func ExampleParseString() {
	graphs, err := dot.ParseString(`
		digraph first { a -> b [label="go"] }
		digraph second { node [shape=box]; x }
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(graphs[0].ID, graphs[0].Edges[0].Label)
	fmt.Println(graphs[1].ID, graphs[1].Node("x").Shape)
	// Output:
	// first go
	// second box
}

func ExampleParseFile() {
	graphs, err := dot.ParseFile("graph.dot")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := layout.Hierarchical(graphs[0], layout.Options{}); err != nil {
		log.Fatal(err)
	}
}

// Write keeps the computed layout as pos attributes, so that Graphviz
// can draw it with "neato -n2".
func ExampleWrite() {
	graph := layout.NewDigraph()
	graph.Edge("a", "b")
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	if err := dot.Write(os.Stdout, l); err != nil {
		log.Fatal(err)
	}
	// Output:
	// digraph {
	// 	bb="0.00,0.00,48.00,112.00";
	// 	"a" [pos="32.00,80.00", width=0.444, height=0.444];
	// 	"b" [pos="32.00,16.00", width=0.444, height=0.444];
	// 	"a" -> "b" [pos="e,32.00,32.00 32.00,64.00 32.00,64.00 32.00,42.00 32.00,42.00"];
	// }
}
