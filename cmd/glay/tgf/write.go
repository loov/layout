package tgf

import (
	"fmt"
	"io"

	"github.com/loov/layout"
)

func WriteLayout(out io.Writer, graph *layout.Graph) error {
	var err error
	write := func(format string, args ...interface{}) bool {
		if err != nil {
			return false
		}

		_, err = fmt.Fprintf(out, format, args...)
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

	return err
}
