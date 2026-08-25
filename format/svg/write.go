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

// writeRecord draws record fields: separators between sub-fields and
// centered text in leaf fields
func (svg *writer) writeRecord(graph *layout.Graph, node *layout.Node, rec *layout.RecordField, origin layout.Vector) {
	if len(rec.Fields) == 0 {
		center := layout.Vector{
			X: origin.X + (rec.TopLeft.X+rec.BottomRight.X)/2,
			Y: origin.Y + (rec.TopLeft.Y+rec.BottomRight.Y)/2,
		}
		svg.writeText(graph, rec.Text, center, node.FontSize, node.FontName, node.FontColor)
		return
	}
	for i, field := range rec.Fields {
		if i > 0 {
			var a, b layout.Vector
			if rec.Vertical {
				a = layout.Vector{X: rec.TopLeft.X, Y: field.TopLeft.Y}
				b = layout.Vector{X: rec.BottomRight.X, Y: field.TopLeft.Y}
			} else {
				a = layout.Vector{X: field.TopLeft.X, Y: rec.TopLeft.Y}
				b = layout.Vector{X: field.TopLeft.X, Y: rec.BottomRight.Y}
			}
			svg.write("<line x1='%v' y1='%v' x2='%v' y2='%v' stroke='%v' stroke-width='%v'/>",
				origin.X+a.X, origin.Y+a.Y, origin.X+b.X, origin.Y+b.Y, dkcolor(node.LineColor), node.LineWidth)
		}
		svg.writeRecord(graph, node, field, origin)
	}
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

// writeLabel writes plain text centered at center, or an HTML-like label
// as a foreignObject filling the box of the given half size.
func (svg *writer) writeLabel(graph *layout.Graph, label string, center, radius layout.Vector, fontSize layout.Length, fontName string, color layout.Color) {
	if !layout.IsHTMLLabel(label) {
		svg.writeText(graph, label, center, fontSize, fontName, color)
		return
	}
	svg.write("<foreignObject x='%v' y='%v' width='%v' height='%v'", center.X-radius.X, center.Y-radius.Y, 2*radius.X, 2*radius.Y)
	if fontSize != 0 {
		svg.write(" font-size='%v'", fontSize)
	}
	if fontName != "" {
		svg.write(" font-family='%v'", fontName)
	}
	svg.write(" color='%v'", dkcolor(color))
	svg.write(`><body xmlns="http://www.w3.org/1999/xhtml" style="margin:0;display:flex;align-items:center;justify-content:center;height:100%%">%v</body>`, lowercaseTags(label[1:len(label)-1]))
	svg.write("</foreignObject>")
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
		svg.writeStroke(edge.LineWidth, edge.LineStyle)
		svg.write(" d='%v'>", roundedPath(edge.Path, 2*graph.RowPadding))

		if edge.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(edge.Tooltip))
		}

		svg.write("</path>")

		if edge.Label != "" {
			svg.writeLabel(graph, edge.Label, edge.LabelPos, edge.LabelRadius, edge.FontSize, edge.FontName, edge.FontColor)
		}
	}

	for _, node := range graph.Nodes {
		svgtag := svg.writeShape(node, node.Radius)
		svg.write(" class='node'")

		svg.write(" fill='%v'", ltcolor(node.FillColor))
		svg.write(" stroke='%v'", dkcolor(node.LineColor))
		svg.writeStroke(node.LineWidth, node.LineStyle)

		svg.write(">")
		if node.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(node.Tooltip))
		}
		svg.write("</%v>", svgtag)

		// extra peripheries are inset outlines without fill
		for i := 1; i < node.Peripheries; i++ {
			inset := layout.Length(i) * peripheryGap
			svg.writeShape(node, node.Radius.Add(layout.Vector{X: -inset, Y: -inset}))
			svg.write(" fill='none'")
			svg.write(" stroke='%v'", dkcolor(node.LineColor))
			svg.writeStroke(node.LineWidth, node.LineStyle)
			svg.write("/>")
		}

		if node.Image != "" {
			svg.write("<image href='%v' x='%v' y='%v' width='%v' height='%v' preserveAspectRatio='xMidYMid meet'/>",
				escapeString(node.Image), node.Left(), node.Top(), 2*node.Radius.X, 2*node.Radius.Y)
		}

		if node.Shape == layout.Record {
			svg.writeRecord(graph, node, graph.LayoutRecord(node), node.TopLeft())
			continue
		}
		if label := node.DefaultLabel(); label != "" {
			svg.writeLabel(graph, label, node.Center, node.Radius, node.FontSize, node.FontName, node.FontColor)
		}
	}
	svg.finishG()
	svg.finish()

	return svg.err
}

// peripheryGap matches layout.peripheryGap
const peripheryGap = 4 * layout.Point

// writeShape opens the node's shape element with the given half size and
// returns the tag name; attributes can follow.
func (svg *writer) writeShape(node *layout.Node, radius layout.Vector) string {
	c := node.Center
	switch node.Shape {
	case layout.Ellipse, layout.Auto:
		svg.write("<ellipse cx='%v' cy='%v' rx='%v' ry='%v'", c.X, c.Y, radius.X, radius.Y)
		return "ellipse"
	case layout.Box, layout.Record:
		svg.write("<rect x='%v' y='%v' width='%v' height='%v'", c.X-radius.X, c.Y-radius.Y, 2*radius.X, 2*radius.Y)
		return "rect"
	case layout.Square:
		r := max(radius.X, radius.Y)
		svg.write("<rect x='%v' y='%v' width='%v' height='%v'", c.X-radius.X, c.Y-radius.Y, 2*r, 2*r)
		return "rect"
	case layout.None:
		svg.write("<g x='%v' y='%v' width='%v' height='%v'", c.X-radius.X, c.Y-radius.Y, 2*radius.X, 2*radius.Y)
		return "g"
	default:
		r := max(radius.X, radius.Y)
		svg.write("<circle cx='%v' cy='%v' r='%v'", c.X, c.Y, r)
		return "circle"
	}
}

// writeStroke writes stroke width and dash attributes for a line style
func (svg *writer) writeStroke(width layout.Length, style layout.LineStyle) {
	switch style {
	case layout.Bold:
		width *= 2
	case layout.Dashed:
		svg.write(" stroke-dasharray='%v'", 5*width)
	case layout.Dotted:
		svg.write(" stroke-dasharray='%v %v' stroke-linecap='round'", width, 2*width)
	}
	svg.write(" stroke-width='%v'", width)
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
