package graph

import (
	"strconv"

	"github.com/loov/layout"
)

type Graph struct {
	ID       string
	Directed bool
	Attrs    Attrs

	Nodes map[string]*Node
	Edges []*Edge

	Errors []error
}

func NewGraph() *Graph {
	graph := &Graph{}
	graph.Nodes = make(map[string]*Node)
	return graph
}

func (graph *Graph) EnsureNode(id string) *Node {
	node := graph.Nodes[id]
	if node == nil {
		node = &Node{}
		node.ID = id
		graph.Nodes[id] = node
	}
	return node
}

type Node struct {
	ID string
	Visual

	LayoutID   layout.ID
	LayoutNode *layout.Node
}

func (node *Node) String() string {
	if node == nil {
		return "?"
	}
	return node.ID
}

type Edge struct {
	Directed bool
	From, To *Node
	Visual
}

func (edge *Edge) String() string {
	return edge.From.String() + "->" + edge.To.String()
}

type Visual struct {
	Attrs Attrs

	Weight float64

	Shape string

	Label    string
	Color    string
	FontSize float64

	PenColor string
	PenWidth float64

	Fill   string
	Width  float64
	Height float64

	Tooltip string
}

type Attrs []Attr
type Attr struct{ Key, Value string }

func (attrs *Attrs) Add(key, value string) { *attrs = append(*attrs, Attr{key, value}) }
func (attrs *Attrs) Append(rest ...Attr)   { *attrs = append(*attrs, rest...) }

func (attrs Attrs) Get(key string) string {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Value
		}
	}
	return ""
}

func (attrs Attrs) Float64(key string, def float64) float64 {
	s := attrs.Get(key)
	if s == "" {
		return def
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}

	return v
}

func (attrs Attrs) String(key string) string { return attrs.Get(key) }
