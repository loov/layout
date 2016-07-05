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
		for _, did := range src.Out {
			write("\t%v -> %v;\n", src.ID, did)
		}
	}
	write("}")
	return
}

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
		for _, did := range src.Out {
			write("%v %v\n", src.ID, did)
		}
	}

	return
}

func (graph *Graph) WriteSVG(out io.Writer) (n int, err error) {
	write := func(format string, args ...interface{}) bool {
		if err != nil {
			return false
		}

		var x int
		x, err = fmt.Fprintf(out, format, args...)
		n += x
		return err == nil
	}

	write("<svg xmlns='http://www.w3.org/2000/svg'>\n")
	defer write("</svg>\n")

	write("\t<g>\n")
	for _, src := range graph.Nodes {
		if src.Virtual {
			continue
		}

		for _, did := range src.Out {
			p := graph.Positions[src.ID]
			write("\t\t<polyline fill='none' stroke='black'")
			write(" points='%v,%v", p.X, p.Y)
			dst := graph.Nodes[did]
			for dst.Virtual {
				p = graph.Positions[dst.ID]
				write(" %v,%v", p.X, p.Y)
				dst = graph.Nodes[dst.Out[0]]
			}
			p = graph.Positions[dst.ID]
			write(" %v,%v'", p.X, p.Y)
			write(" />\n")
		}
	}
	write("\t</g>\n")

	write("\t<g>\n")
	for _, src := range graph.Nodes {
		if src.Virtual {
			continue
		}
		p := graph.Positions[src.ID]
		write("\t\t<circle cx='%v' cy='%v' r='%v'", p.X, p.Y, 5)
		write(" fill='white' stroke='black'")
		write(" />\n")
	}
	write("\t</g>\n")

	return
}
