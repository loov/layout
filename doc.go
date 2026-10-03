// Package layout lays out graphs for drawing.
//
// A Graph is built from Node and Edge values, either directly or by parsing
// a file with one of the format packages. Hierarchical and Force compute a
// Layout: node boxes, edge paths and cluster boxes, index-aligned with the
// graph, which they leave unchanged. The format packages then write the
// layout out, for example as SVG.
//
//	graph := layout.NewDigraph()
//	graph.Edge("A", "B")
//	l, err := layout.Hierarchical(graph, layout.Options{})
//	if err != nil {
//		log.Fatal(err)
//	}
//	svg.Write(os.Stdout, l)
//
// All lengths are in points, see Length.
package layout
