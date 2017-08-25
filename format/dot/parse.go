// package dot implements dot file format parsing
package dot

import (
	"io"
	"strconv"
	"strings"

	"github.com/loov/layout"

	"gonum.org/v1/gonum/graph/formats/dot"
	"gonum.org/v1/gonum/graph/formats/dot/ast"
)

func Parse(r io.Reader) ([]*layout.Graph, error)     { return parse(dot.Parse(r)) }
func ParseFile(path string) ([]*layout.Graph, error) { return parse(dot.ParseFile(path)) }
func ParseString(s string) ([]*layout.Graph, error)  { return parse(dot.ParseString(s)) }

func parse(file *ast.File, err error) ([]*layout.Graph, error) {
	if err != nil {
		return nil, err
	}

	graphs := []*layout.Graph{}
	for _, graphStmt := range file.Graphs {
		parser := &parserContext{}
		parser.Graph = layout.NewGraph()
		parser.parse(graphStmt)
		graphs = append(graphs, parser.Graph)
	}

	return graphs, nil
}

type parserContext struct {
	Graph *layout.Graph

	allAttrs  []*ast.Attr
	nodeAttrs []*ast.Attr
	edgeAttrs []*ast.Attr
}

func (context *parserContext) parse(src *ast.Graph) {
	context.Graph.ID = src.ID
	context.Graph.Directed = src.Directed
	context.parseStmts(src.Stmts)
}

func (context *parserContext) parseStmts(stmts []ast.Stmt) {
	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case *ast.NodeStmt:
			context.parseNode(stmt)
		case *ast.EdgeStmt:
			context.parseEdge(stmt)
		case *ast.AttrStmt:
			switch stmt.Kind {
			case ast.NodeKind:
				context.nodeAttrs = append(context.nodeAttrs, stmt.Attrs...)
			case ast.EdgeKind:
				context.edgeAttrs = append(context.edgeAttrs, stmt.Attrs...)
			case ast.GraphKind:
				context.allAttrs = append(context.allAttrs, stmt.Attrs...)
			default:
				panic("unknown attr target kind")
			}
		case *ast.Attr:
			context.allAttrs = append(context.allAttrs, stmt)
		case *ast.Subgraph:
			subcontext := &parserContext{}
			subcontext.Graph = context.Graph
			subcontext.allAttrs = append(subcontext.allAttrs, context.allAttrs...)
			subcontext.nodeAttrs = append(subcontext.nodeAttrs, context.nodeAttrs...)
			subcontext.edgeAttrs = append(subcontext.edgeAttrs, context.edgeAttrs...)
			subcontext.parseStmts(stmt.Stmts)
		}
	}
}

func (context *parserContext) ensureNode(id string) *layout.Node {
	if node, exists := context.Graph.NodeByID[id]; exists {
		return node
	}

	node := context.Graph.Node(id)
	applyNodeAttrs(node, context.nodeAttrs)
	return node
}

func (context *parserContext) parseNode(src *ast.NodeStmt) *layout.Node {
	node := context.ensureNode(src.Node.ID)
	applyNodeAttrs(node, src.Attrs)
	return node
}

func (context *parserContext) parseEdge(edgeStmt *ast.EdgeStmt) {
	sources := context.ensureVertex(edgeStmt.From)
	to := edgeStmt.To
	for to != nil {
		targets := context.ensureVertex(to.Vertex)
		for _, source := range sources {
			for _, target := range targets {
				edge := &layout.Edge{}

				edge.Directed = to.Directed
				edge.From = source
				edge.To = target

				applyEdgeAttrs(edge, context.edgeAttrs)
				applyEdgeAttrs(edge, edgeStmt.Attrs)

				context.Graph.Edges = append(context.Graph.Edges, edge)
			}
		}

		sources = targets
		to = to.To
	}
}

func (context *parserContext) ensureVertex(src ast.Vertex) []*layout.Node {
	switch src := src.(type) {
	case *ast.Node:
		return []*layout.Node{context.ensureNode(src.ID)}
	case *ast.Subgraph:
		nodes := []*layout.Node{}
		for _, stmt := range src.Stmts {
			switch stmt := stmt.(type) {
			case *ast.NodeStmt:
				nodes = append(nodes, context.parseNode(stmt))
			default:
				panic("unsupported stmt inside subgraph")
			}
		}
		return nodes
	default:
		panic("vertex not supported")
	}
}

func applyNodeAttrs(node *layout.Node, attrs []*ast.Attr) {
	for _, attr := range attrs {
		switch attr.Key {
		case "weight":
			setFloat(&node.Weight, attr.Val)
		case "shape":
			setShape(&node.Shape, attr.Val)
		case "label":
			setString(&node.Label, attr.Val)
		case "color":
			setColor(&node.FontColor, attr.Val)
		case "fontsize":
			setLength(&node.FontSize, attr.Val, layout.Point)
		case "pencolor":
			setColor(&node.PenColor, attr.Val)
		case "penwidth":
			setLength(&node.PenWidth, attr.Val, layout.Point)
		case "fillcolor":
			setColor(&node.FillColor, attr.Val)
		case "width":
			setLength(&node.Radius.X, attr.Val, layout.Inch*0.5)
		case "height":
			setLength(&node.Radius.Y, attr.Val, layout.Inch*0.5)
		case "tooltip":
			setString(&node.Tooltip, attr.Val)
		}
	}
}

func applyEdgeAttrs(edge *layout.Edge, attrs []*ast.Attr) {
	for _, attr := range attrs {
		switch attr.Key {
		case "weight":
			setFloat(&edge.Weight, attr.Val)
		case "label":
			setString(&edge.Label, attr.Val)
		case "color":
			setColor(&edge.FontColor, attr.Val)
		case "fontsize":
			setLength(&edge.FontSize, attr.Val, layout.Point)
		case "pencolor":
			setColor(&edge.PenColor, attr.Val)
		case "penwidth":
			setLength(&edge.PenWidth, attr.Val, layout.Point)
		case "tooltip":
			setString(&edge.Tooltip, attr.Val)
		}
	}
}

func setColor(t *layout.Color, value string) {
	if value == "" {
		return
	}

	if value[0] == '#' { // hex
		value = value[1:]
		if len(value) == 6 { // RRGGBB
			v, err := strconv.ParseInt(value, 16, 64)
			if err == nil {
				c := layout.RGB{}
				c.R = uint8(v >> 16)
				c.G = uint8(v >> 8)
				c.B = uint8(v >> 0)
				*t = c
			}
		} else if len(value) == 8 { // RRGGBBAA
			v, err := strconv.ParseInt(value, 16, 64)
			if err == nil {
				c := layout.RGBA{}
				c.R = uint8(v >> 24)
				c.G = uint8(v >> 16)
				c.B = uint8(v >> 8)
				c.A = uint8(v >> 0)
				*t = c
			}
		}
		return
	}

	color, ok := layout.ColorByName(value)
	if ok {
		*t = color
	}
}

func setFloat(t *float64, value string) {
	v, err := strconv.ParseFloat(value, 64)
	if err == nil {
		*t = v
	}
}

func setLength(t *layout.Length, value string, unit layout.Length) {
	v, err := strconv.ParseFloat(value, 64)
	if err == nil {
		*t = layout.Length(v) * unit
	}
}

func setShape(t *layout.Shape, value string) {
	switch value {
	case "box", "rect", "rectangle":
		*t = layout.Box
	case "square":
		*t = layout.Square
	case "circle":
		*t = layout.Circle
	case "ellipse", "oval":
		*t = layout.Ellipse
	default:
		*t = layout.Auto
	}
}

func setString(t *string, value string) {
	if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	*t = strings.Replace(value, "\\n", "\n", -1)
}
