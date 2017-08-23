package layout

import (
	"fmt"
	"io"
)

func (graph *Graph) WriteTGF(out io.Writer) (n int, err error) {
	write := func(format string, args ...interface{}) bool {
		if err != nil {
			return false
		}

		var x int
		x, err = fmt.Fprintf(out, format, args...)
		n += x
		return err == nil
	}

	for _, src := range graph.Nodes {
		if !src.Virtual {
			write("%v %v\n", src.ID, src.ID)
		} else {
			write("%v\n", src.ID)
		}
	}

	write("#\n")

	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			write("%v %v\n", src.ID, dst.ID)
		}
	}

	return
}
