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

func (svg *writer) start(width, height layout.Length) {
	svg.write("<svg xmlns='http://www.w3.org/2000/svg' width='%v' height='%v'>", width, height)
}
func (svg *writer) finish() {
	svg.write("</svg>\n")
}

func (svg *writer) writeStyle() {
	svg.write(`
	<style type="text/css"><![CDATA[
		.edge { fill: none; }
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

func colortext(color layout.Color) string {
	const hex = "0123456789ABCDEF"
	r, g, b, a := color.RGBA8()
	if a == 0 {
		return "none"
	}
	return string([]byte{'#',
		hex[r>>4], hex[r&7],
		hex[g>>4], hex[g&7],
		hex[b>>4], hex[b&7],
		//hex[a>>4], hex[a&7],
	})
}

func dkcolor(color layout.Color) string {
	if color == nil {
		return "#000000"
	}
	return colortext(color)
}

func ltcolor(color layout.Color) string {
	if color == nil {
		return "#FFFFFF"
	}
	return colortext(color)
}

func Write(w io.Writer, graph *layout.Graph) error {
	svg := &writer{}
	svg.w = w

	_, bottomRight := graph.Bounds()
	svg.start(bottomRight.X+graph.NodePadding, bottomRight.Y+graph.RowPadding)
	svg.writeStyle()
	svg.writeDefs()

	svg.startG()
	for _, edge := range graph.Edges {
		if edge.Directed {
			svg.write("<path class='edge' marker-end='url(#arrowhead)'")
		} else {
			svg.write("<path class='edge'")
		}

		svg.write(" stroke='%v'", dkcolor(edge.LineColor))
		svg.write(" stroke-width='%v'", edge.LineWidth)

		svg.write(" d='")
		p0 := edge.Path[0]
		p1 := edge.Path[1]
		dir := layout.Length(1)
		if p0.Y > p1.Y {
			dir *= -1
		}
		var sx, sy layout.Length
		svg.write("M %v %v ", p0.X, p0.Y)
		for _, p2 := range edge.Path[2:] {
			sx = p0.X*0.2 + p1.X*0.8
			if (p0.X < p1.X) != (p1.X < p2.X) {
				sx = p1.X
			}
			sy = p1.Y - dir*graph.RowPadding
			svg.write("S %v %v %v %v ", sx, sy, p1.X, p1.Y)
			p0, p1 = p1, p2
		}
		sx = p0.X*0.2 + p1.X*0.8
		sy = p1.Y - 2*dir*graph.RowPadding
		svg.write("S %v %v %v %v ", sx, sy, p1.X, p1.Y)
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
		case layout.Circle:
			svgtag = "circle"
			r := max(node.Radius.X, node.Radius.Y)
			svg.write("<circle cx='%v' cy='%v' r='%v'", node.Center.X, node.Center.Y, r)
		case layout.Ellipse, layout.Auto:
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

		svg.write(" fill='%v'", ltcolor(node.FillColor))
		svg.write(" stroke='%v'", dkcolor(node.LineColor))
		svg.write(" stroke-width='%v'", node.LineWidth)

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
				svg.write(" color='%v'", dkcolor(node.FontColor))
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
