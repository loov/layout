package main

import (
	"fmt"
	"io"
	"os"

	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/graphml"
)

func main() {
	var in io.Reader = os.Stdin
	var out io.Writer = os.Stdout

	if len(os.Args) >= 2 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open %v", os.Args[1])
			os.Exit(1)
			return
		}
		in = file
		defer file.Close()
	}

	if len(os.Args) >= 3 {
		file, err := os.Create(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create %v", os.Args[2])
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

	graphml.Write(out, graphs...)
}
