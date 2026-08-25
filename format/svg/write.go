package svg

import (
	"fmt"
	stdhtml "html"
	"io"
	"strconv"
	"strings"

	"github.com/loov/layout"
	"golang.org/x/net/html"
)

type writer struct {
	w   io.Writer
	err error
}

func (svg *writer) erred() bool  { return svg.err != nil }
func (svg *writer) Error() error { return svg.err }

func (svg *writer) write(format string, args ...any) {
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

func vec(x, y layout.Length) string {
	return strconv.FormatFloat(float64(x), 'f', -1, 32) + "," +
		strconv.FormatFloat(float64(y), 'f', -1, 32) + " "
}

// splinePath draws a Catmull-Rom spline through the path points as cubic beziers.
func splinePath(path []layout.Vector) string {
	var line strings.Builder
	line.WriteString("M" + vec(path[0].X, path[0].Y))
	if len(path) == 2 {
		line.WriteString("L" + vec(path[1].X, path[1].Y))
		return line.String()
	}

	at := func(i int) layout.Vector {
		if i < 0 {
			i = 0
		}
		if i >= len(path) {
			i = len(path) - 1
		}
		return path[i]
	}
	for i := 0; i+1 < len(path); i++ {
		p0, p1, p2, p3 := at(i-1), at(i), at(i+1), at(i+2)
		c1 := layout.Vector{X: p1.X + (p2.X-p0.X)/6, Y: p1.Y + (p2.Y-p0.Y)/6}
		c2 := layout.Vector{X: p2.X - (p3.X-p1.X)/6, Y: p2.Y - (p3.Y-p1.Y)/6}
		line.WriteString("C" + vec(c1.X, c1.Y) + vec(c2.X, c2.Y) + vec(p2.X, p2.Y))
	}
	return line.String()
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
		if len(edge.Path) == 0 {
			// TODO: log invalid path
			continue
		}

		if edge.Directed {
			svg.write("<path class='edge' marker-end='url(#arrowhead)'")
		} else {
			svg.write("<path class='edge'")
		}

		svg.write(" stroke='%v'", dkcolor(edge.LineColor))
		svg.write(" stroke-width='%v'", edge.LineWidth)
		svg.write(" d='%v'>", splinePath(edge.Path))

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
		case layout.None:
			svgtag = "g"
			svg.write("<g x='%v' y='%v' width='%v' height='%v'",
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

		if label := node.DefaultLabel(); label != "" {
			if label[0] == '<' && label[len(label)-1] == '>' {
				svg.write("<foreignObject x='%v' y='%v' width='100%%' height='100%%' ", node.Center.X-node.Radius.X, node.Center.Y-node.Radius.Y)
				if node.FontSize != 0 {
					svg.write(" font-size='%v'", node.FontSize)
				}
				if node.FontName != "" {
					svg.write(" font-family='%v'", node.FontName)
				}
				svg.write(" color='%v'", dkcolor(node.FontColor))
				svg.write(`><body xmlns="http://www.w3.org/1999/xhtml">%v</body>`, lowercaseTags(label[1:len(label)-1]))
				svg.write("</foreignObject>")
			} else {
				lines := strings.Split(label, "\n")
				top := node.Center.Y - graph.LineHeight*layout.Length(len(lines))*0.5
				top += graph.LineHeight * 0.5
				for _, line := range lines {
					svg.write("<text text-anchor='middle' alignment-baseline='middle' x='%v' y='%v'", node.Center.X, top)
					if node.FontSize != 0 {
						svg.write(" font-size='%v'", node.FontSize)
					}
					if node.FontName != "" {
						svg.write(" font-family='%v'", node.FontName)
					}
					svg.write(" color='%v'", dkcolor(node.FontColor))
					svg.write(">%v</text>\n", escapeString(line))
					top += graph.LineHeight
				}
			}
		}
	}
	svg.finishG()
	svg.finish()

	return svg.err
}

func lowercaseTags(s string) string {
	root := &html.Node{Type: html.ElementNode}
	nodes, err := html.ParseFragment(strings.NewReader(s), root)
	if err != nil {
		return s
	}

	var out strings.Builder
	for _, node := range nodes {
		err := html.Render(&out, node)
		if err != nil {
			return s
		}
	}
	return out.String()
}

func max(a, b layout.Length) layout.Length {
	if a > b {
		return a
	}
	return b
}

func escapeString(s string) string {
	return stdhtml.EscapeString(s)
}
