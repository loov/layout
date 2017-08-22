package layout

import (
	"fmt"
	"io"
)

func (graph *Graph) EdgeTableString() string {
	n := len(graph.Nodes)
	stride := 2*n + 4
	table := make([]byte, n*stride)
	for i := range table {
		table[i] = ' '
	}

	for id, node := range graph.Nodes {
		row := table[id*stride : (id+1)*stride]
		row[0] = '|'
		row[n+1] = '|'
		row[len(row)-2] = '|'
		row[len(row)-1] = '\n'

		out := row[1 : 1+n]
		for _, dst := range node.Out {
			out[dst] = 'X'
		}

		in := row[1+n+1 : 1+n+1+n]
		for _, src := range node.In {
			in[src] = 'X'
		}
	}

	return string(table)
}

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
	write(`
	<style type="text/css"><![CDATA[
		.node {
			fill: hsla(0, 60%%, 60%%, 0.5);
			stroke: #333;
		}
		.edge {
			fill: none;
			stroke: hsla(180,60%%,30%%,0.5);
		}
		.virtual-node {
			fill: hsla(90, 60%%, 60%%, 0.5);
			stroke: #333;
			stroke-width: 0.1px;
		}
		#head {
			fill: hsla(180,60%%,30%%,0.5);
		}
	]]></style>`)
	write(`
	<defs>
		<marker id='head' orient='auto' markerWidth='2' markerHeight='4' refX='0.0' refY='2'>
			<path d='M0,0 V4 L2,2 Z'/>
		</marker>
	</defs>`)
	defer write("</svg>\n")

	write("\t<g>\n")
	for _, src := range graph.Nodes {
		if src.Virtual {
			continue
		}

		for _, did := range src.Out {
			p := graph.Positions[src.ID]
			write("\t\t<polyline class='edge' marker-end='url(#head)'")
			write(" points='%v,%v", p.X, p.Y+nodesize/2)
			dst := graph.Nodes[did]
			for dst.Virtual {
				p = graph.Positions[dst.ID]
				write(" %v,%v", p.X, p.Y)
				dst = graph.Nodes[dst.Out[0]]
			}
			p = graph.Positions[dst.ID]
			write(" %v,%v'", p.X, p.Y-nodesize/2)
			write(" />\n")
		}
	}
	write("\t</g>\n")

	write("\t<g>\n")
	for _, src := range graph.Nodes {
		p := graph.Positions[src.ID]
		write("\t\t<circle cx='%v' cy='%v'", p.X, p.Y)
		if !src.Virtual {
			write(" r='%v'", nodesize/2)
			write(" class='node'")
		} else {
			write(" r='%v'", 1)
			write(" class='virtual-node'")
		}
		write(" />\n")
	}
	write("\t</g>\n")

	write("\t<g>\n")
	for _, src := range graph.Nodes {
		p := graph.Positions[src.ID]
		if !src.Virtual {
			write("\t\t<text text-anchor='middle' x='%v' y='%v'", p.X, p.Y)
			write(" font-size='%v'", nodesize/2)
			write(">%v</text>\n", src.Label)
		}
	}
	write("\t</g>\n")

	return
}
