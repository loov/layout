package dot

import (
	"fmt"
	"io"
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

	_, size := graph.Bounds()
	flipY := func(v layout.Vector) layout.Vector { return layout.Vector{X: v.X, Y: size.Y - v.Y} }
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
	write("\tbb=\"0,0,%s\";\n", pt(layout.Vector{X: size.X, Y: 0}))
	for _, node := range graph.Nodes {
		attrs := []string{
			"pos=" + quote(pt(node.Center)),
			"width=" + inches(2*node.Radius.X),
			"height=" + inches(2*node.Radius.Y),
		}
		if node.Label != "" {
			attrs = append(attrs, "label="+quote(node.Label))
		}
		if node.Shape != layout.Auto {
			attrs = append(attrs, "shape="+string(node.Shape))
		}
		write("\t%s [%s];\n", quote(node.ID), strings.Join(attrs, ", "))
	}
	for _, edge := range graph.Edges {
		var attrs []string
		if len(edge.Path) >= 2 {
			// Graphviz expects 3n+1 cubic Bezier control points. Repeating
			// each segment's ends represents the polyline without bending it.
			points := []string{pt(edge.Path[0])}
			for i := 1; i < len(edge.Path); i++ {
				points = append(points, pt(edge.Path[i-1]), pt(edge.Path[i]), pt(edge.Path[i]))
			}
			attrs = append(attrs, "pos="+quote(strings.Join(points, " ")))
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

// quote returns s as a dot string literal
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}
