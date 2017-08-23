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

		for _, dst := range src.Out {
			write("\t\t<polyline class='edge' marker-end='url(#head)'")
			write(" points='%v,%v", src.Position.X, src.Position.Y+src.Radius.Y)

			for dst.Virtual {
				write(" %v,%v", dst.Position.X, dst.Position.Y)
				dst = dst.Out[0]
			}

			write(" %v,%v'", dst.Position.X, dst.Position.Y-dst.Radius.Y)
			write(" />\n")
		}
	}
	write("\t</g>\n")

	write("\t<g>\n")
	for _, src := range graph.Nodes {
		write("\t\t<circle cx='%v' cy='%v'", src.Position.X, src.Position.Y)
		if !src.Virtual {
			write(" r='%v'", nodeWidth/2)
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
		if !src.Virtual {
			write("\t\t<text text-anchor='middle' x='%v' y='%v'", src.Position.X, src.Position.Y)
			write(" font-size='%v'", nodeWidth/2)
			write(">%v</text>\n", src.Label)
		}
	}
	write("\t</g>\n")

	return
}
