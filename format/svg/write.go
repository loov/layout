package svg

import (
	"fmt"
	stdhtml "html"
	"io"
	"math"
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
		<marker id="arrowhead" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto" markerUnits="userSpaceOnUse">
	      <path d="M0,0 L0,8 L10,4 z" fill="context-stroke" />
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

// splinePath draws a smooth curve through the path points as cubic beziers.
// Tangents follow the neighboring points, scaled to each segment's own length
// so that a long segment doesn't distort a short one next to it.
func splinePath(path []layout.Vector) string {
	var line strings.Builder
	line.WriteString("M" + vec(path[0].X, path[0].Y))
	if len(path) == 2 {
		line.WriteString("L" + vec(path[1].X, path[1].Y))
		return line.String()
	}

	// unit tangent at each point
	tangent := make([]layout.Vector, len(path))
	for i := range path {
		a, b := path[max(0, i-1)], path[min(len(path)-1, i+1)]
		d := layout.Vector{X: b.X - a.X, Y: b.Y - a.Y}
		if l := math.Hypot(float64(d.X), float64(d.Y)); l > 0 {
			d.X /= layout.Length(l)
			d.Y /= layout.Length(l)
		}
		tangent[i] = d
	}

	for i := 0; i+1 < len(path); i++ {
		p1, p2 := path[i], path[i+1]
		k := layout.Length(math.Hypot(float64(p2.X-p1.X), float64(p2.Y-p1.Y))) / 3
		c1 := layout.Vector{X: p1.X + tangent[i].X*k, Y: p1.Y + tangent[i].Y*k}
		c2 := layout.Vector{X: p2.X - tangent[i+1].X*k, Y: p2.Y - tangent[i+1].Y*k}
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

func escapeString(s string) string {
	return stdhtml.EscapeString(s)
}
