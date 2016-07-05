package glay

import (
	"fmt"
	"io"
)

func (graph *Graph) WriteDOT(out io.Writer) (n int, err error) {
	write := func(format string, args ...interface{}) bool {
		var x int
		x, err = fmt.Fprintf(out, format, args...)
		n += x
		return err == nil
	}

	if !write("digraph G {\n") {
		return
	}

	for _, src := range graph.Nodes {
		if !src.Virtual {
			if !write("\t%v[rank = %v];\n", src.ID, src.Rank) {
				return
			}
		} else {
			if !write("\t%v[rank = %v; shape=circle];\n", src.ID, src.Rank) {
				return
			}
		}
		for _, did := range src.Out {
			if !write("\t%v -> %v;\n", src.ID, did) {
				return
			}
		}
	}

	if !write("}") {
		return
	}
	return
}

func (graph *Graph) WriteTGF(out io.Writer) (n int, err error) {
	write := func(format string, args ...interface{}) bool {
		var x int
		x, err = fmt.Fprintf(out, format, args...)
		n += x
		return err == nil
	}

	for _, src := range graph.Nodes {
		if !src.Virtual {
			if !write("%v %v\n", src.ID, src.ID) {
				return
			}
		} else {
			if !write("%v\n", src.ID) {
				return
			}
		}
	}

	if !write("#\n") {
		return
	}

	for _, src := range graph.Nodes {
		for _, did := range src.Out {
			if !write("%v %v\n", src.ID, did) {
				return
			}
		}
	}

	return
}
