// package dot implements dot file format parsing
package dot

import (
	"io"

	"github.com/loov/layout/cmd/glay/graph"

	"gonum.org/v1/gonum/graph/formats/dot"
	"gonum.org/v1/gonum/graph/formats/dot/ast"
)

func Parse(r io.Reader) ([]*graph.Graph, error)     { return parse(dot.Parse(r)) }
func ParseFile(path string) ([]*graph.Graph, error) { return parse(dot.ParseFile(path)) }
func ParseString(s string) ([]*graph.Graph, error)  { return parse(dot.ParseString(s)) }

func parse(file *ast.File, err error) ([]*graph.Graph, error) {
	if err != nil {
		return nil, err
	}

	graphs := []*graph.Graph{}
	for _, graphStmt := range file.Graphs {
		parser := &parserContext{}
		parser.Graph = graph.NewGraph()
		parser.parse(graphStmt)
		graphs = append(graphs, parser.Graph)
	}

	return graphs, nil
}

type parserContext struct {
	Graph *graph.Graph

	allAttrs  graph.Attrs
	nodeAttrs graph.Attrs
	edgeAttrs graph.Attrs
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
				context.nodeAttrs.Append(attrs(stmt.Attrs)...)
			case ast.EdgeKind:
				context.edgeAttrs.Append(attrs(stmt.Attrs)...)
			case ast.GraphKind:
				context.allAttrs.Append(attrs(stmt.Attrs)...)
			default:
				panic("unknown attr target kind")
			}
		case *ast.Attr:
			context.Graph.Attrs.Add(stmt.Key, stmt.Key)
		case *ast.Subgraph:
			subcontext := &parserContext{}
			subcontext.Graph = context.Graph
			subcontext.nodeAttrs.Append(context.nodeAttrs...)
			subcontext.edgeAttrs.Append(context.edgeAttrs...)
			subcontext.parseStmts(stmt.Stmts)
		}
	}
}

func (context *parserContext) ensureNode(id string) *graph.Node {
	node, ok := context.Graph.Nodes[id]
	if !ok {
		node = &graph.Node{}
		node.ID = id
		node.Attrs.Append(context.nodeAttrs...)
		update(&node.Visual)
		context.Graph.Nodes[id] = node
	}
	return node
}

func (context *parserContext) parseNode(src *ast.NodeStmt) *graph.Node {
	node := context.ensureNode(src.Node.ID)
	if len(src.Attrs) > 0 {
		node.Attrs.Append(attrs(src.Attrs)...)
		update(&node.Visual)
	}
	return node
}

func attrs(attrs []*ast.Attr) graph.Attrs {
	xs := graph.Attrs{}
	for _, attr := range attrs {
		xs.Add(attr.Key, attr.Val)
	}
	return xs
}

func (context *parserContext) parseEdge(edgeStmt *ast.EdgeStmt) {
	sources := context.ensureVertex(edgeStmt.From)
	to := edgeStmt.To
	for to != nil {
		targets := context.ensureVertex(to.Vertex)
		for _, source := range sources {
			for _, target := range targets {
				edge := &graph.Edge{}

				edge.Directed = to.Directed
				edge.From = source
				edge.To = target

				edge.Attrs.Append(context.edgeAttrs...)
				edge.Attrs.Append(attrs(edgeStmt.Attrs)...)
				update(&edge.Visual)

				context.Graph.Edges = append(context.Graph.Edges, edge)
			}
		}

		sources = targets
		to = to.To
	}
}

func (context *parserContext) ensureVertex(src ast.Vertex) []*graph.Node {
	switch src := src.(type) {
	case *ast.Node:
		return []*graph.Node{context.ensureNode(src.ID)}
	case *ast.Subgraph:
		nodes := []*graph.Node{}
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

func update(visual *graph.Visual) {
	visual.Weight = visual.Attrs.Float64("weight", 1)
	visual.Shape = visual.Attrs.String("shape")
	visual.Label = visual.Attrs.String("label")
	visual.Color = visual.Attrs.String("color")
	visual.FontSize = visual.Attrs.Float64("fontsize", 14)
	visual.PenColor = visual.Attrs.String("pencolor")
	visual.PenWidth = visual.Attrs.Float64("penwidth", 1)
	visual.Fill = visual.Attrs.String("fillcolor")
	visual.Width = visual.Attrs.Float64("width", 0)
	visual.Height = visual.Attrs.Float64("height", 0)
	visual.Tooltip = visual.Attrs.String("tooltip")
}
