// Package layout lays out graphs for drawing.
//
// A Graph is built from Node and Edge values, either directly or by parsing
// a file with one of the format packages. Hierarchical assigns coordinates to
// the nodes and paths to the edges; the format packages then write the result
// out, for example as SVG.
//
//	graph := layout.NewDigraph()
//	graph.Edge("A", "B")
//	layout.Hierarchical(graph)
//	svg.Write(os.Stdout, graph)
//
// All lengths are in points, see Length.
package layout
