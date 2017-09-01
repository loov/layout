package graphml

import (
	"bytes"
	"encoding/xml"
	"io"

	"github.com/loov/layout"
)

func Write(out io.Writer, graphs ...*layout.Graph) error {
	file := NewFile()
	for _, graph := range graphs {
		file.Graphs = append(file.Graphs, Convert(graph))
	}

	enc := xml.NewEncoder(out)
	enc.Indent("", "\t")
	return enc.Encode(file)
}

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
		addAttr(&outnode.Attrs, "label", node.DefaultLabel())
		addAttr(&outnode.Attrs, "shape", string(node.Shape))
		addAttr(&outnode.Attrs, "tooltip", node.Tooltip)
		out.Node = append(out.Node, outnode)
	}

	for _, edge := range graph.Edges {
		outedge := Edge{}
		outedge.Source = edge.From.ID
		outedge.Target = edge.To.ID
		addAttr(&outedge.Attrs, "label", edge.Label)
		addAttr(&outedge.Attrs, "tooltip", edge.Tooltip)
		out.Edge = append(out.Edge, outedge)
	}

	return out
}

func addAttr(attrs *[]Attr, key, value string) {
	if value == "" {
		return
	}
	*attrs = append(*attrs, Attr{key, escapeText(value)})
}

func escapeText(s string) []byte {
	if s == "" {
		return []byte{}
	}

	var buf bytes.Buffer
	err := xml.EscapeText(&buf, []byte(s))
	if err != nil {
		// this shouldn't ever happen
		panic(err)
	}
	return buf.Bytes()
}
