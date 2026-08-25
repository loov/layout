package graphml

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/loov/layout"
)

// Parse reads a GraphML document and returns every top-level graph in it.
// Node and edge "label" and node "shape" data keys are applied; graphs
// nested inside nodes are flattened into their parent.
func Parse(r io.Reader) ([]*layout.Graph, error) {
	var file File
	if err := xml.NewDecoder(r).Decode(&file); err != nil {
		return nil, err
	}

	// data keys may use ids different from their attribute names
	keyName := map[string]string{}
	for _, key := range file.Key {
		name := key.AttrName
		if name == "" {
			name = key.ID
		}
		keyName[key.ID] = name
	}

	var graphs []*layout.Graph
	for _, src := range file.Graphs {
		graph := layout.NewGraph()
		graph.ID = src.ID
		graph.Directed = src.EdgeDefault == Directed
		if err := convertGraph(graph, src, keyName); err != nil {
			return nil, err
		}
		graphs = append(graphs, graph)
	}
	return graphs, nil
}

// ParseFile reads a GraphML file and returns every top-level graph in it.
func ParseFile(path string) ([]*layout.Graph, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

func convertGraph(graph *layout.Graph, src *Graph, keyName map[string]string) error {
	for i, srcnode := range src.Node {
		if srcnode.ID == "" {
			return fmt.Errorf("graph %q: node %d has no id", src.ID, i)
		}
		node := graph.Node(srcnode.ID)
		for _, attr := range srcnode.Attrs {
			value := attrText(attr)
			switch keyName[attr.Key] {
			case "label":
				node.Label = value
			case "shape":
				node.Shape = layout.Shape(value)
			case "tooltip":
				node.Tooltip = value
			}
		}
		for _, sub := range srcnode.Graph {
			if err := convertGraph(graph, sub, keyName); err != nil {
				return err
			}
		}
	}
	for i, srcedge := range src.Edge {
		if srcedge.Source == "" || srcedge.Target == "" {
			return fmt.Errorf("graph %q: edge %d has an empty endpoint", src.ID, i)
		}
		edge := layout.NewEdge(graph.Node(srcedge.Source), graph.Node(srcedge.Target))
		edge.Directed = graph.Directed
		if srcedge.Directed != nil {
			edge.Directed = *srcedge.Directed
		}
		for _, attr := range srcedge.Attrs {
			value := attrText(attr)
			switch keyName[attr.Key] {
			case "label":
				edge.Label = value
			case "tooltip":
				edge.Tooltip = value
			}
		}
		graph.AddEdge(edge)
	}
	return nil
}

// attrText returns the plain text of a data element, unescaping the inner xml
func attrText(attr Attr) string {
	var text strings.Builder
	dec := xml.NewDecoder(strings.NewReader(string(attr.Value)))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if chars, ok := tok.(xml.CharData); ok {
			text.Write(chars)
		}
	}
	return text.String()
}
