// Package svg writes laid out graphs as SVG images.
package svg

import (
	"cmp"
	"fmt"
	stdhtml "html"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
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

func (svg *writer) start(left, top, right, bottom layout.Length) {
	svg.write("<svg xmlns='http://www.w3.org/2000/svg' width='%v' height='%v'", right-left, bottom-top)
	if left != 0 || top != 0 { // content at negative coordinates needs a shifted viewport
		svg.write(" viewBox='%v %v %v %v'", left, top, right-left, bottom-top)
	}
	svg.write(">")
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

// colortext returns color as #RRGGBB, or as rgba() when translucent
func colortext(color layout.Color) string {
	const hex = "0123456789ABCDEF"
	r, g, b, a := color.RGBA8()
	if a == 0 {
		return "none"
	}
	if a != 0xFF {
		return fmt.Sprintf("rgba(%d,%d,%d,%.3g)", r, g, b, float64(a)/0xFF)
	}
	return string([]byte{'#',
		hex[r>>4], hex[r&0xF],
		hex[g>>4], hex[g&0xF],
		hex[b>>4], hex[b&0xF],
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
func roundedPath(path []layout.Vector, radius, maxDeviation layout.Length) string {
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
		if radius <= 0 {
			line.WriteString("L" + vec(p.X, p.Y))
			continue
		}
		r := layout.Length(draw.CornerRadius(drawPoint(prev), drawPoint(p), drawPoint(next), float64(radius), float64(maxDeviation)))
		in, out := towards(p, prev, r), towards(p, next, r)
		line.WriteString("L" + vec(in.X, in.Y))
		line.WriteString("Q" + vec(p.X, p.Y) + vec(out.X, out.Y))
	}
	last := path[len(path)-1]
	line.WriteString("L" + vec(last.X, last.Y))
	return line.String()
}

// markerID returns the marker definition for an arrow style, or "" for
// none; styles without a marker of their own draw a normal arrowhead
func markerID(arrow layout.Arrow) string {
	switch arrow {
	case layout.ArrowDefault, layout.ArrowNone:
		return ""
	case layout.ArrowVee, layout.ArrowDot, layout.ArrowODot:
		return string(arrow)
	}
	return string(layout.ArrowNormal)
}

// drawPoint converts v for the draw package
func drawPoint(v layout.Vector) draw.Point { return draw.Point{X: float64(v.X), Y: float64(v.Y)} }

// layoutRecord computes the record fields of a node, measuring text as
// the layout did
func layoutRecord(graph *layout.Graph, node *layout.Node, box layout.NodeBox) *draw.Record {
	var lineWidth func(string) float64
	if graph.MeasureText != nil {
		lineWidth = func(line string) float64 { return float64(graph.MeasureText(line, node.FontName, box.FontSize)) }
	}
	sideways := graph.RankDir == layout.LeftToRight || graph.RankDir == layout.RightToLeft
	return draw.LayoutRecord(box.Label, sideways, float64(box.Size.X), float64(box.Size.Y), float64(graph.LineHeight), float64(box.FontSize), lineWidth)
}

// writeRecord draws record fields: separators between sub-fields and
// centered text in leaf fields
func (svg *writer) writeRecord(graph *layout.Graph, node *layout.Node, box layout.NodeBox, rec *draw.Record, origin layout.Vector) {
	if len(rec.Fields) == 0 {
		center := layout.Vector{
			X: origin.X + layout.Length(rec.X0+rec.X1)/2,
			Y: origin.Y + layout.Length(rec.Y0+rec.Y1)/2,
		}
		half := layout.Length(rec.X1-rec.X0) / 2
		svg.writeText(graph, rec.Text, center, half, box.FontSize, node.FontName, node.FontColor)
		return
	}
	for i, field := range rec.Fields {
		if i > 0 {
			var a, b layout.Vector
			if rec.Vertical {
				a = layout.Vector{X: layout.Length(rec.X0), Y: layout.Length(field.Y0)}
				b = layout.Vector{X: layout.Length(rec.X1), Y: layout.Length(field.Y0)}
			} else {
				a = layout.Vector{X: layout.Length(field.X0), Y: layout.Length(rec.Y0)}
				b = layout.Vector{X: layout.Length(field.X0), Y: layout.Length(rec.Y1)}
			}
			svg.write("<line x1='%v' y1='%v' x2='%v' y2='%v' stroke='%v' stroke-width='%v'/>",
				origin.X+a.X, origin.Y+a.Y, origin.X+b.X, origin.Y+b.Y, dkcolor(node.LineColor), node.LineWidth)
		}
		svg.writeRecord(graph, node, box, field, origin)
	}
}

// writeText writes multi-line text around center: a line centered on it,
// or against the left or right side of a box half wide, a font size in,
// see draw.Lines
func (svg *writer) writeText(graph *layout.Graph, text string, center layout.Vector, half, fontSize layout.Length, fontName string, color layout.Color) {
	lines := draw.Lines(text)
	top := center.Y - graph.LineHeight*layout.Length(len(lines))*0.5
	top += graph.LineHeight * 0.5
	inset := max(half-cmp.Or(fontSize, graph.FontSize)*0.5, 0)
	for _, line := range lines {
		anchor, x := "middle", center.X
		switch line.Align {
		case draw.Left:
			anchor, x = "start", center.X-inset
		case draw.Right:
			anchor, x = "end", center.X+inset
		}
		svg.write("<text text-anchor='%v' alignment-baseline='middle' x='%v' y='%v'", anchor, x, top)
		if fontSize != 0 {
			svg.write(" font-size='%v'", fontSize)
		}
		if fontName != "" {
			svg.write(" font-family='%v'", escapeString(fontName))
		}
		svg.write(" fill='%v'", dkcolor(color))
		svg.write(">%v</text>\n", escapeString(line.Text))
		top += graph.LineHeight
	}
}

// writeLabel writes plain text centered at center, or an HTML-like label
// as a foreignObject filling the box of the given half size.
func (svg *writer) writeLabel(graph *layout.Graph, label string, center, radius layout.Vector, fontSize layout.Length, fontName string, color layout.Color) {
	if !draw.IsHTMLLabel(label) {
		svg.writeText(graph, label, center, radius.X, fontSize, fontName, color)
		return
	}
	svg.write("<foreignObject x='%v' y='%v' width='%v' height='%v'", center.X-radius.X, center.Y-radius.Y, 2*radius.X, 2*radius.Y)
	if fontSize != 0 {
		svg.write(" font-size='%v'", fontSize)
	}
	if fontName != "" {
		svg.write(" font-family='%v'", escapeString(fontName))
	}
	svg.write(" color='%v'", dkcolor(color))
	svg.write(`><body xmlns="http://www.w3.org/1999/xhtml" style="margin:0;display:flex;align-items:center;justify-content:center;height:100%%">%v</body>`, sanitizeHTML(label[1:len(label)-1]))
	svg.write("</foreignObject>")
}

// Write renders the laid out graph as an SVG document.
//
// Nodes are drawn according to their shape and colors, edges as rounded
// polylines along their path with an arrowhead on directed edges. Labels
// wrapped in <...> are emitted as inline HTML.
func Write(w io.Writer, l *layout.Layout) error {
	svg := &writer{}
	svg.w = w
	graph := l.Graph

	topLeft, bottomRight := l.Bounds()
	svg.start(min(topLeft.X, 0), min(topLeft.Y, 0), bottomRight.X+graph.NodePadding, bottomRight.Y+graph.RowPadding)
	svg.writeStyle()
	svg.writeDefs()

	svg.startG()
	for i, cluster := range graph.Clusters {
		if cluster.Invisible {
			continue
		}
		box := l.Clusters[i]
		svg.write("<rect class='cluster' x='%v' y='%v' width='%v' height='%v'",
			box.TopLeft.X, box.TopLeft.Y,
			box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)
		svg.write(" fill='%v'", ltcolor(cluster.FillColor))
		svg.write(" stroke='%v'", dkcolor(cluster.LineColor))
		svg.write("></rect>")
		if cluster.Label != "" {
			center := layout.Vector{X: (box.TopLeft.X + box.BottomRight.X) / 2, Y: box.TopLeft.Y + graph.LineHeight/2}
			radius := layout.Vector{X: (box.BottomRight.X - box.TopLeft.X) / 2, Y: graph.LineHeight / 2}
			svg.writeLabel(graph, cluster.Label, center, radius, graph.FontSize, "", nil)
		}
	}

	for i, edge := range graph.Edges {
		if edge.Invisible {
			continue
		}
		path := l.Edges[i]
		if len(path.Path) == 0 {
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
		radius := 2 * graph.RowPadding
		if graph.Splines == layout.SplinesPolyline || graph.Splines == layout.SplinesOrtho {
			radius = 0
		}
		svg.write(" d='%v'>", roundedPath(path.Path, radius, graph.EdgePadding))

		if edge.Tooltip != "" {
			svg.write("<title>%v</title>", escapeString(edge.Tooltip))
		}

		svg.write("</path>")

		if edge.Label != "" {
			labelRadius := layout.Vector{X: path.LabelSize.X / 2, Y: path.LabelSize.Y / 2}
			svg.writeLabel(graph, edge.Label, path.LabelCenter, labelRadius, path.FontSize, edge.FontName, edge.FontColor)
		}
	}

	for i, node := range graph.Nodes {
		if node.Invisible {
			continue
		}
		box := l.Nodes[i]
		radius := layout.Vector{X: box.Size.X / 2, Y: box.Size.Y / 2}
		svgtag := svg.writeShape(box, radius)
		svg.write(" class='node'")

		fill := ltcolor(node.FillColor)
		if box.Shape == layout.PointShape && node.FillColor == nil {
			fill = dkcolor(node.LineColor) // points are solid
		}
		svg.write(" fill='%v'", fill)
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
			svg.writeShape(box, radius.Add(layout.Vector{X: -inset, Y: -inset}))
			svg.write(" fill='none'")
			svg.write(" stroke='%v'", dkcolor(node.LineColor))
			svg.writeStroke(node.LineWidth, node.LineStyle)
			svg.write("/>")
		}

		if node.Image != "" {
			svg.write("<image href='%v' x='%v' y='%v' width='%v' height='%v' preserveAspectRatio='xMidYMid meet'/>",
				escapeString(node.Image), box.Left(), box.Top(), box.Size.X, box.Size.Y)
		}

		if box.Shape == layout.Record {
			svg.writeRecord(graph, node, box, layoutRecord(graph, node, box), box.TopLeft())
			continue
		}
		if box.Label != "" {
			svg.writeLabel(graph, box.Label, box.Center, radius, box.FontSize, node.FontName, node.FontColor)
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
func (svg *writer) writeShape(box layout.NodeBox, radius layout.Vector) string {
	c := box.Center
	switch box.Shape {
	case layout.Ellipse, layout.Auto:
		svg.write("<ellipse cx='%v' cy='%v' rx='%v' ry='%v'", c.X, c.Y, radius.X, radius.Y)
		return "ellipse"
	case layout.Box, layout.Record:
		svg.write("<rect x='%v' y='%v' width='%v' height='%v'", c.X-radius.X, c.Y-radius.Y, 2*radius.X, 2*radius.Y)
		return "rect"
	case layout.Square:
		r := max(radius.X, radius.Y)
		svg.write("<rect x='%v' y='%v' width='%v' height='%v'", c.X-r, c.Y-r, 2*r, 2*r)
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

// sanitizeHTML normalizes an HTML-like label into well-formed XHTML and
// strips anything that could run script: script-like elements, on*
// handlers and javascript urls.
func sanitizeHTML(s string) string {
	root := &html.Node{Type: html.ElementNode}
	nodes, err := html.ParseFragment(strings.NewReader(s), root)
	if err != nil {
		return escapeString(s)
	}

	for _, node := range nodes {
		root.AppendChild(node)
	}
	sanitizeNode(root)
	var out strings.Builder
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		renderXML(&out, node)
	}
	return out.String()
}

// renderXML writes n as XML. Comments and other non-element nodes are
// dropped, and so are attributes whose name is not an XML name without a
// namespace prefix; elements with such names are replaced by their
// content.
func renderXML(out *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		out.WriteString(escapeString(n.Data))
		return
	case html.ElementNode:
	default:
		return
	}
	named := xmlName(n.Data)
	if named {
		out.WriteString("<" + n.Data)
		seen := map[string]bool{}
		for _, a := range n.Attr {
			if a.Namespace == "" && xmlName(a.Key) && !seen[a.Key] {
				seen[a.Key] = true
				out.WriteString(" " + a.Key + `="` + escapeString(a.Val) + `"`)
			}
		}
		if n.FirstChild == nil {
			out.WriteString("/>")
			return
		}
		out.WriteString(">")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderXML(out, c)
	}
	if named {
		out.WriteString("</" + n.Data + ">")
	}
}

// xmlName reports whether s is an XML name without a namespace prefix
func xmlName(s string) bool {
	for i, r := range s {
		if !(unicode.IsLetter(r) || r == '_' || i > 0 && (unicode.IsDigit(r) || r == '-' || r == '.')) {
			return false
		}
	}
	return s != ""
}

func sanitizeNode(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode {
			switch c.Data {
			case "script", "style", "iframe", "object", "embed", "link", "meta", "base", "form", "svg", "math",
				// raw text elements are rendered unescaped, so their text would
				// become markup when the SVG is read as XML
				"noscript", "xmp", "noembed", "noframes", "plaintext":
				n.RemoveChild(c)
				c = next
				continue
			}
		}
		sanitizeNode(c)
		c = next
	}
	if n.Type != html.ElementNode {
		return
	}
	n.Attr = slices.DeleteFunc(n.Attr, func(a html.Attribute) bool {
		key := strings.ToLower(a.Key)
		// URL parsing strips ASCII tabs and newlines anywhere in a URL,
		// and leading/trailing C0 controls and spaces before reading its scheme.
		val := strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(a.Val)
		val = strings.ToLower(strings.TrimFunc(val, func(r rune) bool { return r <= 0x20 }))
		return strings.HasPrefix(key, "on") ||
			((key == "href" || key == "src" || key == "xlink:href" || key == "action" || key == "formaction") &&
				(strings.HasPrefix(val, "javascript:") || strings.HasPrefix(val, "data:")))
	})
}

// escapeString escapes s for XML text and attribute values, dropping
// characters that XML 1.0 does not allow
func escapeString(s string) string {
	return stdhtml.EscapeString(strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || 0x20 <= r && r <= 0xD7FF || 0xE000 <= r && r <= 0xFFFD || 0x10000 <= r {
			return r
		}
		return -1
	}, s))
}
