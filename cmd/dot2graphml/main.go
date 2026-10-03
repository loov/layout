// Command dot2graphml converts dot files to GraphML that yEd can open.
//
// Usage:
//
//	dot2graphml [-erase-labels] [-set-shape shape] input.dot [output.graphml]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/graphml"
)

var (
	eraseLabels = flag.Bool("erase-labels", false, "erase custom labels")
	setShape    = flag.String("set-shape", "", "override default shape")
)

func main() {
	flag.Parse()
	args := flag.Args()

	var graphs []*layout.Graph
	var err error
	if len(args) >= 1 {
		graphs, err = dot.ParseFile(args[0])
	} else {
		graphs, err = dot.Parse(os.Stdin)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse input:", err)
		os.Exit(1)
		return
	}

	// the output is created only after parsing, as it may be the input
	var out io.Writer = os.Stdout
	if len(args) >= 2 {
		filename := args[1]
		file, err := os.Create(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create %v: %v\n", filename, err)
			os.Exit(1)
			return
		}
		out = file
		defer file.Close()
	}

	for _, graph := range graphs {
		for _, node := range graph.Nodes {
			if *eraseLabels {
				node.Label = ""
			}
			if *setShape != "" {
				node.Shape = layout.Shape(*setShape)
			}
		}
	}

	if err := graphml.Write(out, graphs...); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write output:", err)
		os.Exit(1)
	}
}
