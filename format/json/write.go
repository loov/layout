// Package json writes laid out graphs as plain JSON: node centers and
// sizes, edge paths and label positions, cluster boxes. It is the stable
// coordinate export for tools that draw the graph themselves.
package json

import (
	"encoding/json"
	"io"

	"github.com/loov/layout"
)

// Graph is the JSON shape of a laid out graph. The drawing spans Width
// and Height from the top left corner X, Y. Coordinates are kept as laid
// out, so X and Y are 0 unless something lies before the origin, as with
// negative pinned positions.
type Graph struct {
	ID       string    `json:"id,omitempty"`
	Directed bool      `json:"directed"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Width    float64   `json:"width"`
	Height   float64   `json:"height"`
	Nodes    []Node    `json:"nodes"`
	Edges    []Edge    `json:"edges"`
	Clusters []Cluster `json:"clusters,omitempty"`
}

// Node is a node with its center at X, Y and its full Width and Height.
type Node struct {
	ID     string  `json:"id"`
	Label  string  `json:"label,omitempty"`
	Shape  string  `json:"shape,omitempty"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Edge is an edge drawn through the points of Path. LabelPos is the
// center of the label, when the edge has one.
type Edge struct {
	From     string       `json:"from"`
	To       string       `json:"to"`
	Directed bool         `json:"directed"`
	Label    string       `json:"label,omitempty"`
	Path     [][2]float64 `json:"path"`
	LabelPos *[2]float64  `json:"labelPos,omitempty"`
}

// Cluster is the box drawn around Nodes, given by their IDs. Unlike
// Node, X and Y are the top left corner. Parent is the ID of the
// enclosing cluster, if any.
type Cluster struct {
	ID     string   `json:"id"`
	Label  string   `json:"label,omitempty"`
	Parent string   `json:"parent,omitempty"`
	Nodes  []string `json:"nodes"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Width  float64  `json:"width"`
	Height float64  `json:"height"`
}

// Convert builds the JSON shape of a laid out graph.
func Convert(graph *layout.Graph) Graph {
	lo, hi := graph.Bounds()
	lo = layout.Vector{X: min(lo.X, 0), Y: min(lo.Y, 0)}
	out := Graph{ID: graph.ID, Directed: graph.Directed,
		X: float64(lo.X), Y: float64(lo.Y),
		Width:  float64(hi.X + graph.NodePadding - lo.X),
		Height: float64(hi.Y + graph.RowPadding - lo.Y),
		Nodes:  []Node{}, Edges: []Edge{}}
	for _, node := range graph.Nodes {
		out.Nodes = append(out.Nodes, Node{
			ID: node.ID, Label: node.Label, Shape: string(node.Shape),
			X: float64(node.Center.X), Y: float64(node.Center.Y),
			Width: float64(2 * node.Radius.X), Height: float64(2 * node.Radius.Y),
		})
	}
	for _, edge := range graph.Edges {
		e := Edge{From: edge.From.ID, To: edge.To.ID, Directed: edge.Directed, Label: edge.Label, Path: [][2]float64{}}
		for _, p := range edge.Path {
			e.Path = append(e.Path, [2]float64{float64(p.X), float64(p.Y)})
		}
		if edge.Label != "" {
			e.LabelPos = &[2]float64{float64(edge.LabelPos.X), float64(edge.LabelPos.Y)}
		}
		out.Edges = append(out.Edges, e)
	}
	for _, cluster := range graph.Clusters {
		c := Cluster{ID: cluster.ID, Label: cluster.Label, Nodes: []string{},
			X: float64(cluster.TopLeft.X), Y: float64(cluster.TopLeft.Y),
			Width:  float64(cluster.BottomRight.X - cluster.TopLeft.X),
			Height: float64(cluster.BottomRight.Y - cluster.TopLeft.Y)}
		if cluster.Parent != nil {
			c.Parent = cluster.Parent.ID
		}
		for _, node := range cluster.Nodes {
			c.Nodes = append(c.Nodes, node.ID)
		}
		out.Clusters = append(out.Clusters, c)
	}
	return out
}

// Write writes the laid out graph as indented JSON.
func Write(w io.Writer, graph *layout.Graph) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(Convert(graph))
}
