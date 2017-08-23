package layout

import (
	"fmt"
	"io"
)

func (graph *Graph) WriteDOT(out io.Writer) (n int, err error) {
	write := func(format string, args ...interface{}) bool {
		if err != nil {
			return false
		}
		var x int
		x, err = fmt.Fprintf(out, format, args...)
		n += x
		return err == nil
	}

	write("digraph G {\n")
	for _, src := range graph.Nodes {
		if !src.Virtual {
			write("\t%v[rank = %v];\n", src.ID, src.Rank)
		} else {
			write("\t%v[rank = %v; shape=circle];\n", src.ID, src.Rank)
		}
		for _, dst := range src.Out {
			write("\t%v -> %v;\n", src.ID, dst.ID)
		}
	}
	write("}")
	return
}
