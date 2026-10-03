package dot

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
)

// Write writes the graphs as dot with the computed layout, one after
// another: nodes carry pos, width and height, edges carry pos with cubic
// controls for their paths. Labels, colors, line styles, ports and
// clusters are kept. The output can be rendered by Graphviz with
// "neato -n2".
//
// Coordinates are in points with the y axis pointing up, as in Graphviz.
func Write(w io.Writer, graphs ...*layout.Graph) error {
	for _, graph := range graphs {
		if err := writeGraph(w, graph); err != nil {
			return err
		}
	}
	return nil
}

// writeGraph writes one graph, see Write
func writeGraph(w io.Writer, graph *layout.Graph) error {
	var err error
	write := func(format string, args ...any) {
		if err == nil {
			_, err = fmt.Fprintf(w, format, args...)
		}
	}

	// the drawing starts at the origin unless it reaches before it
	lo, hi := graph.Bounds()
	lo = layout.Vector{X: min(lo.X, 0), Y: min(lo.Y, 0)}
	// mirroring within the bounding box keeps coordinates in place
	flipY := func(v layout.Vector) layout.Vector { return layout.Vector{X: v.X, Y: lo.Y + hi.Y - v.Y} }
	pt := func(v layout.Vector) string {
		v = flipY(v)
		return strconv.FormatFloat(float64(v.X), 'f', 2, 64) + "," + strconv.FormatFloat(float64(v.Y), 'f', 2, 64)
	}
	inches := func(l layout.Length) string { return strconv.FormatFloat(float64(l)/layout.Inch, 'f', 3, 64) }

	kind := "graph"
	arrow := "--"
	if graph.Directed {
		kind, arrow = "digraph", "->"
	}
	if graph.ID != "" {
		write("%s %s {\n", kind, quote(graph.ID))
	} else {
		write("%s {\n", kind)
	}
	if graph.RankDir != layout.TopToBottom {
		write("\trankdir=%s;\n", graph.RankDir)
	}
	write("\tbb=\"%s,%s\";\n", pt(layout.Vector{X: lo.X, Y: hi.Y}), pt(layout.Vector{X: hi.X, Y: lo.Y}))
	for _, node := range graph.Nodes {
		attrs := []string{
			"pos=" + quote(pt(node.Center)),
			"width=" + inches(2*node.Radius.X),
			"height=" + inches(2*node.Radius.Y),
		}
		if node.Label != "" || node.NoLabel {
			attrs = append(attrs, "label="+labelID(node.Label))
		}
		if node.Shape != layout.Auto {
			attrs = append(attrs, "shape="+quote(string(node.Shape)))
		}
		if node.Peripheries > 1 {
			attrs = append(attrs, fmt.Sprintf("peripheries=%d", node.Peripheries))
		}
		if node.FillColor != nil {
			attrs = append(attrs, "fillcolor="+colorID(node.FillColor))
		}
		attrs = append(attrs, lineAttrs(node.LineColor, node.FontColor, node.LineWidth, node.LineStyle, node.FillColor != nil)...)
		write("\t%s [%s];\n", quote(node.ID), strings.Join(attrs, ", "))
	}
	for _, edge := range graph.Edges {
		var attrs []string
		head, tail := edge.ArrowHead, edge.ArrowTail
		if head == layout.ArrowDefault && edge.Directed {
			head = layout.ArrowNormal
		}
		hasHead := head != layout.ArrowDefault && head != layout.ArrowNone
		hasTail := tail != layout.ArrowDefault && tail != layout.ArrowNone
		if len(edge.Path) >= 2 {
			// Graphviz ends the spline at the arrow base and gives the tip
			// separately with "s," and "e,".
			path := slices.Clone(edge.Path)
			var ends []string
			if hasTail {
				ends = append(ends, "s,"+pt(path[0]))
				path[0] = arrowBase(path[0], path[1])
			}
			if hasHead {
				n := len(path) - 1
				ends = append(ends, "e,"+pt(path[n]))
				path[n] = arrowBase(path[n], path[n-1])
			}
			// Graphviz expects 3n+1 cubic Bezier control points. Repeating
			// each segment's ends represents the polyline without bending it.
			points := append(ends, pt(path[0]))
			for i := 1; i < len(path); i++ {
				points = append(points, pt(path[i-1]), pt(path[i]), pt(path[i]))
			}
			attrs = append(attrs, "pos="+quote(strings.Join(points, " ")))
		}
		dir := "none"
		switch {
		case hasHead && hasTail:
			dir = "both"
		case hasHead:
			dir = "forward"
		case hasTail:
			dir = "back"
		}
		defaultDir := "none"
		if graph.Directed {
			defaultDir = "forward"
		}
		if dir != defaultDir {
			attrs = append(attrs, "dir="+dir)
		}
		if hasHead && head != layout.ArrowNormal {
			attrs = append(attrs, "arrowhead="+quote(string(head)))
		}
		if hasTail && tail != layout.ArrowNormal {
			attrs = append(attrs, "arrowtail="+quote(string(tail)))
		}
		if edge.Label != "" {
			attrs = append(attrs, "label="+labelID(edge.Label), "lp="+quote(pt(edge.LabelPos)))
		}
		if edge.Weight != 1 {
			attrs = append(attrs, "weight="+strconv.FormatFloat(edge.Weight, 'g', -1, 64))
		}
		if edge.FromPort != layout.CompassAuto {
			attrs = append(attrs, "tailport="+quote(string(edge.FromPort)))
		}
		if edge.ToPort != layout.CompassAuto {
			attrs = append(attrs, "headport="+quote(string(edge.ToPort)))
		}
		attrs = append(attrs, lineAttrs(edge.LineColor, edge.FontColor, edge.LineWidth, edge.LineStyle, false)...)
		if len(attrs) == 0 {
			write("\t%s %s %s;\n", quote(edge.From.ID), arrow, quote(edge.To.ID))
			continue
		}
		write("\t%s %s %s [%s];\n", quote(edge.From.ID), arrow, quote(edge.To.ID), strings.Join(attrs, ", "))
	}
	var writeCluster func(cluster *layout.Cluster, indent string)
	writeCluster = func(cluster *layout.Cluster, indent string) {
		// Graphviz draws only subgraphs named cluster* as clusters
		id := cluster.ID
		if id == "" {
			id = strconv.Itoa(slices.Index(graph.Clusters, cluster))
		}
		if !strings.HasPrefix(id, "cluster") {
			id = "cluster_" + id
		}
		write("%ssubgraph %s {\n", indent, quote(id))
		if cluster.Label != "" {
			write("%s\tlabel=%s;\n", indent, labelID(cluster.Label))
		}
		if cluster.LineColor != nil {
			write("%s\tcolor=%s;\n", indent, colorID(cluster.LineColor))
		}
		if cluster.FillColor != nil {
			write("%s\tbgcolor=%s;\n", indent, colorID(cluster.FillColor))
		}
		for _, inner := range graph.Clusters {
			if inner.Parent == cluster {
				writeCluster(inner, indent+"\t")
			}
		}
		for _, node := range cluster.Nodes {
			write("%s\t%s;\n", indent, quote(node.ID))
		}
		write("%s}\n", indent)
	}
	for _, cluster := range graph.Clusters {
		if cluster.Parent == nil {
			writeCluster(cluster, "\t")
		}
	}
	write("}\n")
	return err
}

// lineAttrs returns the color, fontcolor, penwidth and style attributes
// of a node or edge; filled adds the filled style
func lineAttrs(line, font layout.Color, width layout.Length, style layout.LineStyle, filled bool) []string {
	var attrs, styles []string
	if line != nil {
		attrs = append(attrs, "color="+colorID(line))
	}
	if font != nil {
		attrs = append(attrs, "fontcolor="+colorID(font))
	}
	if width != layout.Point {
		attrs = append(attrs, "penwidth="+strconv.FormatFloat(float64(width/layout.Point), 'g', -1, 64))
	}
	if filled {
		styles = append(styles, "filled")
	}
	if style != layout.Solid {
		styles = append(styles, string(style))
	}
	if len(styles) > 0 {
		attrs = append(attrs, "style="+quote(strings.Join(styles, ",")))
	}
	return attrs
}

// colorID returns color as a quoted "#rrggbb" or "#rrggbbaa"
func colorID(color layout.Color) string {
	r, g, b, a := color.RGBA8()
	if a == 0xFF {
		return fmt.Sprintf(`"#%02x%02x%02x"`, r, g, b)
	}
	return fmt.Sprintf(`"#%02x%02x%02x%02x"`, r, g, b, a)
}

// labelID returns a label as a dot HTML string when it is HTML-like with
// nesting angle brackets, which keep it in one piece, and quoted otherwise
func labelID(label string) string {
	if !draw.IsHTMLLabel(label) {
		return quote(label)
	}
	depth := 0
	for i, c := range label {
		switch c {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 && i != len(label)-1 {
				return quote(label)
			}
		}
	}
	if depth != 0 {
		return quote(label)
	}
	return label
}

// arrowLength is the Graphviz arrow length at the default arrowsize.
const arrowLength = 10 * layout.Point

// arrowBase returns where an arrow with its tip at tip ends, moving toward
// next by the arrow length but at most halfway, so both ends fit.
func arrowBase(tip, next layout.Vector) layout.Vector {
	d := next.Sub(tip)
	n := layout.Length(math.Hypot(float64(d.X), float64(d.Y)))
	if n == 0 {
		return tip
	}
	t := min(arrowLength/n, 0.5)
	return layout.Vector{X: tip.X + d.X*t, Y: tip.Y + d.Y*t}
}

// quote returns s as a dot string literal
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}
