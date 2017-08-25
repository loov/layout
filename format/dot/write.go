package dot

import (
	"fmt"
	"io"

	"github.com/loov/layout/internal/hier"
)

func writeLayout(out io.Writer, graph *hier.Graph) error {
	var err error

	write := func(format string, args ...interface{}) bool {
		if err != nil {
			return false
		}
		_, err = fmt.Fprintf(out, format, args...)
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
	return err
}
