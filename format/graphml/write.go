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

	file.Key = []Key{
		Key{For: "node", ID: "label", AttrName: "label", AttrType: "string"},
		Key{For: "node", ID: "shape", AttrName: "shape", AttrType: "string"},
		Key{For: "edge", ID: "label", AttrName: "label", AttrType: "string"},

		Key{For: "node", ID: "ynodelabel", YFilesType: "nodegraphics"},
		Key{For: "edge", ID: "yedgelabel", YFilesType: "edgegraphics"},
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
		addYedLabelAttr(&outnode.Attrs, "ynodelabel", node.DefaultLabel())
		out.Node = append(out.Node, outnode)
	}

	for _, edge := range graph.Edges {
		outedge := Edge{}
		outedge.Source = edge.From.ID
		outedge.Target = edge.To.ID
		addAttr(&outedge.Attrs, "label", edge.Label)
		addAttr(&outedge.Attrs, "tooltip", edge.Tooltip)
		addYedLabelAttr(&outedge.Attrs, "yedgelabel", edge.Label)
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

func addYedLabelAttr(attrs *[]Attr, key, value string) {
	if value == "" {
		return
	}
	var buf bytes.Buffer
	buf.WriteString(`<y:ShapeNode><y:NodeLabel>`)
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		// this shouldn't ever happen
		panic(err)
	}
	buf.WriteString(`</y:NodeLabel></y:ShapeNode>`)
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
