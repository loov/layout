// Package svg writes laid out graphs as SVG images.
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

// writer accumulates output and the first error encountered
type writer struct {
	w   io.Writer
	err error
}

// erred reports whether a write has failed
func (svg *writer) erred() bool { return svg.err != nil }

// Error returns the first write error, if any
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
		<marker id="normal" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	      <path d="M0,0 L0,8 L10,4 z" fill="context-stroke" />
	    </marker>
		<marker id="vee" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	      <path d="M0,0 L10,4 L0,8 L3,4 z" fill="context-stroke" />
	    </marker>
		<marker id="dot" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	      <circle cx="4" cy="4" r="3.5" fill="context-stroke" />
	    </marker>
		<marker id="odot" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto-start-reverse" markerUnits="userSpaceOnUse">
	      <circle cx="4" cy="4" r="3" fill="white" stroke="context-stroke" />
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

// roundedPath draws the path as straight segments with rounded corners.
// Each corner is a quadratic curve with the vertex as control point, cut at
// most radius away from the vertex (less if the adjacent segments are short).
func roundedPath(path []layout.Vector, radius layout.Length) string {
	var line strings.Builder
	line.WriteString("M" + vec(path[0].X, path[0].Y))

	length := func(a, b layout.Vector) layout.Length {
		return layout.Length(math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y)))
	}
	// point at distance d from a towards b
	towards := func(a, b layout.Vector, d layout.Length) layout.Vector {
		l := length(a, b)
		if l == 0 {
			return a
		}
		return layout.Vector{X: a.X + (b.X-a.X)*d/l, Y: a.Y + (b.Y-a.Y)*d/l}
	}

	for i := 1; i+1 < len(path); i++ {
		prev, p, next := path[i-1], path[i], path[i+1]
		r := min(radius, length(prev, p)/2, length(p, next)/2)
		in, out := towards(p, prev, r), towards(p, next, r)
		line.WriteString("L" + vec(in.X, in.Y))
		line.WriteString("Q" + vec(p.X, p.Y) + vec(out.X, out.Y))
	}
	last := path[len(path)-1]
	line.WriteString("L" + vec(last.X, last.Y))
	return line.String()
}

// Write renders the laid out graph as an SVG document.
//
// Nodes are drawn according to their shape and colors, edges as rounded
// polylines along Edge.Path with an arrowhead on directed edges. Labels
// wrapped in <...> are emitted as inline HTML.
// markerID returns the marker definition for an arrow style, or "" for none
func markerID(arrow layout.Arrow) string {
	switch arrow {
	case layout.ArrowNormal, layout.ArrowVee, layout.ArrowDot, layout.ArrowODot:
		return string(arrow)
	}
	return ""
}

// writeText writes multi-line text centered on center
func (svg *writer) writeText(graph *layout.Graph, text string, center layout.Vector, fontSize layout.Length, fontName string, color layout.Color) {
	lines := strings.Split(text, "\n")
	top := center.Y - graph.LineHeight*layout.Length(len(lines))*0.5
	top += graph.LineHeight * 0.5
	for _, line := range lines {
		svg.write("<text text-anchor='middle' alignment-baseline='middle' x='%v' y='%v'", center.X, top)
		if fontSize != 0 {
			svg.write(" font-size='%v'", fontSize)
		}
		if fontName != "" {
			svg.write(" font-family='%v'", fontName)
		}
		svg.write(" color='%v'", dkcolor(color))
		svg.write(">%v</text>\n", escapeString(line))
		top += graph.LineHeight
	}
}

func Write(w io.Writer, graph *layout.Graph) error {
	svg := &writer{}
	svg.w = w

	_, bottomRight := graph.Bounds()
	svg.start(bottomRight.X+graph.NodePadding, bottomRight.Y+graph.RowPadding)
	svg.writeStyle()
	svg.writeDefs()

	svg.startG()
	for _, cluster := range graph.Clusters {
		svg.write("<rect class='cluster' x='%v' y='%v' width='%v' height='%v'",
			cluster.TopLeft.X, cluster.TopLeft.Y,
			cluster.BottomRight.X-cluster.TopLeft.X, cluster.BottomRight.Y-cluster.TopLeft.Y)
		svg.write(" fill='%v'", ltcolor(cluster.FillColor))
		svg.write(" stroke='%v'", dkcolor(cluster.LineColor))
		svg.write("></rect>")
		if cluster.Label != "" {
			center := layout.Vector{X: (cluster.TopLeft.X + cluster.BottomRight.X) / 2, Y: cluster.TopLeft.Y + graph.LineHeight/2}
			svg.writeText(graph, cluster.Label, center, graph.FontSize, "", nil)
		}
	}

	for _, edge := range graph.Edges {
		if len(edge.Path) == 0 {
			// TODO: log invalid path
			continue
		}

		svg.write("<path class='edge'")
		head, tail := edge.ArrowHead, edge.ArrowTail
		if head == layout.ArrowDefault && edge.Directed {
			head = layout.ArrowNormal
		}
		if marker := markerID(head); marker != "" {
			svg.write(" marker-end='url(#%v)'", marker)
		}
		if marker := markerID(tail); marker != "" {
			svg.write(" marker-start='url(#%v)'", marker)
		}

		svg.write(" stroke='%v'", dkcolor(edge.LineColor))
		svg.write(" stroke-width='%v'", edge.LineWidth)
		svg.write(" d='%v'>", roundedPath(edge.Path, 2*graph.RowPadding))

		if edge.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(edge.Tooltip))
		}

		svg.write("</path>")

		if edge.Label != "" {
			svg.writeText(graph, edge.Label, edge.LabelPos, edge.FontSize, edge.FontName, edge.FontColor)
		}
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
				svg.writeText(graph, label, node.Center, node.FontSize, node.FontName, node.FontColor)
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
