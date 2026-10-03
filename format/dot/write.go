package dot

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/loov/layout"
)

// Write writes the graph as dot with the computed layout: nodes carry
// pos, width and height, edges carry pos with cubic controls for their
// paths. The output can be rendered by Graphviz with "neato -n2".
//
// Coordinates are in points with the y axis pointing up, as in Graphviz.
func Write(w io.Writer, graph *layout.Graph) error {
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
			attrs = append(attrs, "label="+quote(node.Label))
		}
		if node.Shape != layout.Auto {
			attrs = append(attrs, "shape="+quote(string(node.Shape)))
		}
		if node.Peripheries > 1 {
			attrs = append(attrs, fmt.Sprintf("peripheries=%d", node.Peripheries))
		}
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
			attrs = append(attrs, "label="+quote(edge.Label), "lp="+quote(pt(edge.LabelPos)))
		}
		if edge.Weight != 1 {
			attrs = append(attrs, "weight="+strconv.FormatFloat(edge.Weight, 'g', -1, 64))
		}
		if len(attrs) == 0 {
			write("\t%s %s %s;\n", quote(edge.From.ID), arrow, quote(edge.To.ID))
			continue
		}
		write("\t%s %s %s [%s];\n", quote(edge.From.ID), arrow, quote(edge.To.ID), strings.Join(attrs, ", "))
	}
	write("}\n")
	return err
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
