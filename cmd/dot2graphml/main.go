package main

import (
	"flag"
	"fmt"
	"io"
	"os"

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

	var in io.Reader = os.Stdin
	var out io.Writer = os.Stdout

	if len(args) >= 1 {
		filename := args[0]
		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open %v", filename)
			os.Exit(1)
			return
		}
		in = file
		defer file.Close()
	}

	if len(args) >= 2 {
		filename := args[1]
		file, err := os.Create(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create %v", filename)
			os.Exit(1)
			return
		}
		out = file
		defer file.Close()
	}

	graphs, err := dot.Parse(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		fmt.Fprintln(os.Stderr, "failed to parse input")
		os.Exit(1)
		return
	}

	if *eraseLabels {
		for _, graph := range graphs {
			for _, node := range graph.Nodes {
				node.Label = ""
			}
		}
	}

	graphml.Write(out, graphs...)
}
