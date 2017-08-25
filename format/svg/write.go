package svg

import (
	"fmt"
	"html"
	"io"

	"github.com/loov/layout"
)

type writer struct {
	w   io.Writer
	err error
}

func (svg *writer) erred() bool  { return svg.err != nil }
func (svg *writer) Error() error { return svg.err }

func (svg *writer) write(format string, args ...interface{}) {
	if svg.erred() {
		return
	}
	_, svg.err = fmt.Fprintf(svg.w, format, args...)
}

func (svg *writer) start() {
	svg.write("<svg xmlns='http://www.w3.org/2000/svg'>")
}
func (svg *writer) finish() {
	svg.write("</svg>\n")
}

func (svg *writer) writeStyle() {
	svg.write(`
	<style type="text/css"><![CDATA[
		.node {
			fill: #fff;
			stroke: #000;
		}
		.edge {
			fill: none;
			stroke: #000;
		}
		#head {
			fill: #000;
		}
	]]></style>`)
}

func (svg *writer) startG()  { svg.write("<g>") }
func (svg *writer) finishG() { svg.write("</g>") }

func (svg *writer) writeDefs() {
	svg.write(`
	<defs>
		<marker id='head' orient='auto' markerWidth='2' markerHeight='4' refX='0.0' refY='2'>
			<path d='M0,0 V4 L2,2 Z'/>
		</marker>
	</defs>`)
}

func Write(w io.Writer, graph *layout.Graph) error {
	svg := &writer{}
	svg.w = w

	svg.start()
	svg.writeStyle()
	svg.writeDefs()

	svg.startG()
	for _, edge := range graph.Edges {
		if edge.Directed {
			svg.write("<polyline class='edge' marker-end='url(#head)'")
		} else {
			svg.write("<polyline class='edge'")
		}

		svg.write(" points='")
		for _, p := range edge.Path {
			svg.write("%v,%v ", p.X, p.Y)
		}
		svg.write("'>")

		if edge.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(edge.Tooltip))
		}

		svg.write("</polyline>")
	}

	for _, node := range graph.Nodes {
		// TODO: add other shapes
		svg.write("<circle cx='%v' cy='%v'", node.Center.X, node.Center.Y)
		svg.write(" r='%v'", (node.Radius.X+node.Radius.Y)*0.5)
		svg.write(" class='node'")
		svg.write(">")
		if node.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(node.Tooltip))
		}
		svg.write("</circle>")

		if node.Label != "" {
			svg.write("<text text-anchor='middle' alignment-baseline='middle' x='%v' y='%v'", node.Center.X, node.Center.Y)
			svg.write(" font-size='%v'", node.FontSize)
			svg.write(">%v</text>\n", escapeString(node.Label))
		}
	}
	svg.finishG()
	svg.finish()

	return svg.err
}

func escapeString(s string) string {
	return html.EscapeString(s)
}
