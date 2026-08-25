// Package json writes laid out graphs as plain JSON: node centers and
// sizes, edge paths and label positions, cluster boxes. It is the stable
// coordinate export for tools that draw the graph themselves.
package json

import (
	"encoding/json"
	"io"

	"github.com/loov/layout"
)

// Graph is the JSON shape of a laid out graph.
type Graph struct {
	ID       string    `json:"id,omitempty"`
	Directed bool      `json:"directed"`
	Width    float64   `json:"width"`
	Height   float64   `json:"height"`
	Nodes    []Node    `json:"nodes"`
	Edges    []Edge    `json:"edges"`
	Clusters []Cluster `json:"clusters,omitempty"`
}

type Node struct {
	ID     string  `json:"id"`
	Label  string  `json:"label,omitempty"`
	Shape  string  `json:"shape,omitempty"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Edge struct {
	From     string       `json:"from"`
	To       string       `json:"to"`
	Directed bool         `json:"directed"`
	Label    string       `json:"label,omitempty"`
	Path     [][2]float64 `json:"path"`
	LabelPos *[2]float64  `json:"labelPos,omitempty"`
}

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
	_, size := graph.Bounds()
	out := Graph{ID: graph.ID, Directed: graph.Directed,
		Width:  float64(size.X + graph.NodePadding),
		Height: float64(size.Y + graph.RowPadding),
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
