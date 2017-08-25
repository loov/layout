package svg

import (
	"fmt"
	"html"
	"io"
	"strings"

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
		#arrowhead {
			fill: #000;
		}
	]]></style>`)
}

func (svg *writer) startG()  { svg.write("<g>") }
func (svg *writer) finishG() { svg.write("</g>") }

func (svg *writer) writeDefs() {
	svg.write(`
	<defs>
		<marker id="arrowhead" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto" markerUnits="strokeWidth">
	      <path d="M0,0 L0,6 L9,3 z" />
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
			svg.write("<path class='edge' marker-end='url(#arrowhead)'")
		} else {
			svg.write("<path class='edge'")
		}

		svg.write(" d='")
		p0 := edge.Path[0]
		svg.write("M %v %v ", p0.X, p0.Y)
		for _, p := range edge.Path[1:] {
			py := p.Y - graph.RowPadding
			if p0.Y > p.Y {
				py = p.Y + graph.RowPadding
			}
			px := p0.X*0.2 + p.X*0.8
			svg.write("S %v %v %v %v ", px, py, p.X, p.Y)
			p0 = p
		}
		svg.write("'>")

		if edge.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(edge.Tooltip))
		}

		svg.write("</path>")
	}

	for _, node := range graph.Nodes {
		// TODO: add other shapes
		svgtag := "circle"
		switch node.Shape {
		default:
			fallthrough
		case layout.Circle, layout.Auto:
			svgtag = "circle"
			r := max(node.Radius.X, node.Radius.Y)
			svg.write("<circle cx='%v' cy='%v' r='%v'", node.Center.X, node.Center.Y, r)
		case layout.Ellipse:
			svgtag = "ellipse"
			svg.write("<ellipse cx='%v' cy='%v' rx='%v' ry='%v'",
				node.Center.X, node.Center.Y,
				node.Radius.X, node.Radius.Y)
		case layout.Box:
			svgtag = "rect"
			svg.write("<rect x='%v' y='%v' width='%v' height='%v'",
				node.Center.X-node.Radius.X, node.Center.Y-node.Radius.Y,
				2*node.Radius.X, 2*node.Radius.Y)
		case layout.Square:
			svgtag = "rect"
			r := max(node.Radius.X, node.Radius.Y)
			svg.write("<rect x='%v' y='%v' width='%v' height='%v'",
				node.Center.X-node.Radius.X, node.Center.Y-node.Radius.Y,
				2*r, 2*r)
		}
		svg.write(" class='node'")
		svg.write(">")
		if node.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(node.Tooltip))
		}
		svg.write("</%v>", svgtag)

		if node.Label != "" {
			lines := strings.Split(node.Label, "\n")
			top := node.Center.Y - graph.LineHeight*layout.Length(len(lines))*0.5
			top += graph.LineHeight * 0.5
			for _, line := range lines {
				svg.write("<text text-anchor='middle' alignment-baseline='middle' x='%v' y='%v'", node.Center.X, top)
				svg.write(" font-size='%v'", node.FontSize)
				svg.write(">%v</text>\n", escapeString(line))
				top += graph.LineHeight
			}
		}
	}
	svg.finishG()
	svg.finish()

	return svg.err
}

func max(a, b layout.Length) layout.Length {
	if a > b {
		return a
	}
	return b
}

func escapeString(s string) string {
	return html.EscapeString(s)
}
