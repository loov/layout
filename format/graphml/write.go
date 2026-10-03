// Package graphml writes graphs as GraphML, including the yFiles
// extensions needed for yEd to show labels.
package graphml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"

	"github.com/loov/layout"
)

// Write encodes the graphs as a single GraphML document.
func Write(out io.Writer, graphs ...*layout.Graph) error {
	file := NewFile()
	for _, graph := range graphs {
		file.Graphs = append(file.Graphs, Convert(graph))
	}

	file.Key = []Key{
		{For: "node", ID: "d0", AttrName: "label", AttrType: "string"},
		{For: "node", ID: "d1", AttrName: "shape", AttrType: "string"},
		{For: "node", ID: "d2", AttrName: "tooltip", AttrType: "string"},
		{For: "edge", ID: "d3", AttrName: "label", AttrType: "string"},
		{For: "edge", ID: "d4", AttrName: "tooltip", AttrType: "string"},

		{For: "node", ID: "d5", YFilesType: "nodegraphics"},
		{For: "edge", ID: "d6", YFilesType: "edgegraphics"},
	}

	enc := xml.NewEncoder(out)
	enc.Indent("", "\t")
	return enc.Encode(file)
}

// Convert translates a layout graph into its GraphML representation.
func Convert(graph *layout.Graph) *Graph {
	out := &Graph{}
	out.ID = graph.ID
	if graph.Directed {
		out.EdgeDefault = Directed
	} else {
		out.EdgeDefault = Undirected
	}

	for _, node := range graph.Nodes {
		outnode := Node{}
		outnode.ID = node.ID
		addAttr(&outnode.Attrs, "d0", node.DefaultLabel())
		addAttr(&outnode.Attrs, "d1", string(node.Shape))
		addAttr(&outnode.Attrs, "d2", node.Tooltip)
		addYedAttr(&outnode.Attrs, "d5", "y:ShapeNode", "y:NodeLabel", node.DefaultLabel())
		out.Node = append(out.Node, outnode)
	}

	for _, edge := range graph.Edges {
		outedge := Edge{}
		outedge.Source = edge.From.ID
		outedge.Target = edge.To.ID
		if directed := edgeDirected(edge); directed != graph.Directed {
			outedge.Directed = &directed
		}
		addAttr(&outedge.Attrs, "d3", edge.Label)
		addAttr(&outedge.Attrs, "d4", edge.Tooltip)
		addYedAttr(&outedge.Attrs, "d6", "y:PolyLineEdge", "y:EdgeLabel", edge.Label)
		out.Edge = append(out.Edge, outedge)
	}

	return out
}

// edgeDirected reports whether an edge is drawn directed: an arrow at the
// head only, as with dot's dir=forward, makes it directed and no arrows
// make it undirected. Arrows at the tail cannot be told apart in GraphML
// and keep Edge.Directed.
func edgeDirected(edge *layout.Edge) bool {
	if edge.ArrowTail != layout.ArrowDefault && edge.ArrowTail != layout.ArrowNone {
		return edge.Directed
	}
	switch edge.ArrowHead {
	case layout.ArrowDefault:
		return edge.Directed
	case layout.ArrowNone:
		return false
	}
	return true
}

func addAttr(attrs *[]Attr, key, value string) {
	if value == "" {
		return
	}
	*attrs = append(*attrs, Attr{key, escapeText(value)})
}

func addYedAttr(attrs *[]Attr, key, shape, label, value string) {
	if value == "" {
		return
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<%s><%s>", shape, label)
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		// this shouldn't ever happen
		panic(err)
	}
	fmt.Fprintf(&buf, "</%s></%s>", label, shape)
	*attrs = append(*attrs, Attr{key, buf.Bytes()})
}

func escapeText(s string) []byte {
	if s == "" {
		return []byte{}
	}

	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		// this shouldn't ever happen
		panic(err)
	}
	return buf.Bytes()
}
