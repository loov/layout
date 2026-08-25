// Package dot parses the Graphviz dot file format into layout graphs.
//
// Node and edge attributes that map onto layout properties (label, shape,
// colors, font, line width, tooltip) are applied; rank=same subgraphs are
// recorded in Graph.SameRank. Other attributes are ignored.
package dot

import (
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/loov/layout"

	"gonum.org/v1/gonum/graph/formats/dot"
	"gonum.org/v1/gonum/graph/formats/dot/ast"
)

// Parse reads dot from r and returns every graph it contains.
func Parse(r io.Reader) ([]*layout.Graph, error) { return parse(dot.Parse(r)) }

// ParseFile reads dot from the file at path and returns every graph it contains.
func ParseFile(path string) ([]*layout.Graph, error) { return parse(dot.ParseFile(path)) }

// ParseString parses dot from s and returns every graph it contains.
func ParseString(s string) ([]*layout.Graph, error) { return parse(dot.ParseString(s)) }

// parse converts a parsed dot file into layout graphs
func parse(file *ast.File, err error) ([]*layout.Graph, error) {
	if err != nil {
		return nil, err
	}

	graphs := []*layout.Graph{}
	for _, graphStmt := range file.Graphs {
		parser := &parserContext{positioned: map[*layout.Node]bool{}}
		parser.Graph = layout.NewGraph()
		parser.parse(graphStmt)
		graphs = append(graphs, parser.Graph)
	}

	return graphs, nil
}

// parserContext holds the attribute defaults in effect for a (sub)graph
type parserContext struct {
	Graph   *layout.Graph
	Cluster string

	allAttrs  []*ast.Attr
	nodeAttrs []*ast.Attr
	edgeAttrs []*ast.Attr

	touched []*layout.Node // nodes referenced in this (sub)graph

	positioned map[*layout.Node]bool // nodes with a pos attribute
}

func (context *parserContext) parse(src *ast.Graph) {
	context.Graph.ID = src.ID
	context.Graph.Directed = src.Directed
	context.parseStmts(src.Stmts)
	applyGraphAttrs(context.Graph, context.allAttrs)
	context.pin()
}

// pin marks the graph as positioned when every node has a pos and flips
// the y axis from dot's upwards to ours.
func (context *parserContext) pin() {
	graph := context.Graph
	if len(graph.Nodes) == 0 {
		return
	}
	top := layout.Length(math.Inf(-1))
	for _, node := range graph.Nodes {
		if !context.positioned[node] {
			return
		}
		top = max(top, node.Center.Y+node.Radius.Y)
	}
	// the bounding box, when present, gives the exact height
	for _, attr := range context.allAttrs {
		if attr.Key == "bb" {
			if corners := strings.Split(fixstring(attr.Val), ","); len(corners) == 4 {
				if h, err := strconv.ParseFloat(corners[3], 64); err == nil {
					top = layout.Length(h) * layout.Point
				}
			}
		}
	}
	graph.Pinned = true
	flip := func(v *layout.Vector) { v.Y = top - v.Y }
	for _, node := range graph.Nodes {
		flip(&node.Center)
	}
	for _, edge := range graph.Edges {
		for i := range edge.Path {
			flip(&edge.Path[i])
		}
		if edge.LabelPos != (layout.Vector{}) {
			flip(&edge.LabelPos)
		}
	}
}

// notePos remembers nodes that got a pos attribute
func (context *parserContext) notePos(node *layout.Node, attrs []*ast.Attr) {
	for _, attr := range attrs {
		if attr.Key == "pos" {
			context.positioned[node] = true
		}
	}
}

// parsePoint reads "x,y" in points, ignoring a trailing "!"
func parsePoint(s string) (layout.Vector, bool) {
	x, y, ok := strings.Cut(strings.TrimSuffix(fixstring(s), "!"), ",")
	if !ok {
		return layout.Vector{}, false
	}
	fx, errx := strconv.ParseFloat(x, 64)
	fy, erry := strconv.ParseFloat(y, 64)
	return layout.Vector{X: layout.Length(fx) * layout.Point, Y: layout.Length(fy) * layout.Point}, errx == nil && erry == nil
}

// parseSpline reads an edge pos: points separated by spaces, where "s,x,y"
// and "e,x,y" are the start and end points
func parseSpline(s string) []layout.Vector {
	var start, end *layout.Vector
	var points []layout.Vector
	for field := range strings.FieldsSeq(fixstring(s)) {
		prefix := ""
		if len(field) > 2 && field[1] == ',' && (field[0] == 's' || field[0] == 'e') {
			prefix, field = field[:1], field[2:]
		}
		p, ok := parsePoint(field)
		if !ok {
			continue
		}
		switch prefix {
		case "s":
			start = &p
		case "e":
			end = &p
		default:
			points = append(points, p)
		}
	}
	if start != nil {
		points = append([]layout.Vector{*start}, points...)
	}
	if end != nil {
		points = append(points, *end)
	}
	return points
}

// applyGraphAttrs applies graph level attributes
func applyGraphAttrs(graph *layout.Graph, attrs []*ast.Attr) {
	for _, attr := range attrs {
		switch attr.Key {
		case "rankdir":
			switch strings.ToUpper(fixstring(attr.Val)) {
			case "LR":
				graph.RankDir = layout.LeftToRight
			case "RL":
				graph.RankDir = layout.RightToLeft
			case "BT":
				graph.RankDir = layout.BottomToTop
			default:
				graph.RankDir = layout.TopToBottom
			}
		case "splines":
			switch strings.ToLower(fixstring(attr.Val)) {
			case "polyline":
				graph.Splines = layout.SplinesPolyline
			case "line", "false":
				graph.Splines = layout.SplinesLine
			case "ortho":
				graph.Splines = layout.SplinesOrtho
			}
		case "nodesep":
			setLength(&graph.NodePadding, attr.Val, layout.Inch)
		case "ranksep":
			setLength(&graph.RowPadding, attr.Val, layout.Inch)
		}
	}
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
			subcontext := &parserContext{positioned: context.positioned}
			subcontext.Graph = context.Graph
			subcontext.allAttrs = append(subcontext.allAttrs, context.allAttrs...)
			subcontext.nodeAttrs = append(subcontext.nodeAttrs, context.nodeAttrs...)
			subcontext.edgeAttrs = append(subcontext.edgeAttrs, context.edgeAttrs...)
			start := len(context.Graph.Clusters)
			subcontext.parseStmts(stmt.Stmts)
			for _, node := range subcontext.touched {
				if !slices.Contains(context.touched, node) {
					context.touched = append(context.touched, node)
				}
			}
			if strings.HasPrefix(stmt.ID, "cluster") && len(subcontext.touched) > 0 {
				cluster := &layout.Cluster{ID: stmt.ID, Nodes: subcontext.touched}
				// outer before inner, so inner clusters are drawn on top
				for _, inner := range context.Graph.Clusters[start:] {
					if inner.Parent == nil {
						inner.Parent = cluster
					}
				}
				context.Graph.Clusters = slices.Insert(context.Graph.Clusters, start, cluster)
				var color layout.Color
				filled := false
				for _, attr := range subgraphAttrs(stmt.Stmts) {
					switch attr.Key {
					case "label":
						setString(&cluster.Label, attr.Val)
					case "color":
						setColor(&color, attr.Val)
						setColor(&cluster.LineColor, attr.Val)
					case "pencolor":
						setColor(&cluster.LineColor, attr.Val)
					case "fillcolor", "bgcolor":
						setColor(&cluster.FillColor, attr.Val)
					case "style":
						filled = strings.Contains(attr.Val, "filled")
					}
				}
				if filled && cluster.FillColor == nil {
					cluster.FillColor = color
				}
			}
			switch {
			case hasAttr(stmt.Stmts, "rank", "same"):
				if len(subcontext.touched) > 1 {
					context.Graph.SameRank = append(context.Graph.SameRank, subcontext.touched)
				}
			case hasAttr(stmt.Stmts, "rank", "min"), hasAttr(stmt.Stmts, "rank", "source"):
				context.Graph.MinRank = append(context.Graph.MinRank, subcontext.touched...)
			case hasAttr(stmt.Stmts, "rank", "max"), hasAttr(stmt.Stmts, "rank", "sink"):
				context.Graph.MaxRank = append(context.Graph.MaxRank, subcontext.touched...)
			}
		}
	}
}

func (context *parserContext) ensureNode(id string) *layout.Node {
	node, exists := context.Graph.NodeByID[id]
	if !exists {
		node = context.Graph.Node(fixstring(id))
		node.Label = node.ID // dot's default label is the id, label="" is empty
		applyNodeAttrs(node, context.nodeAttrs)
		context.notePos(node, context.nodeAttrs)
	}
	if !slices.Contains(context.touched, node) {
		context.touched = append(context.touched, node)
	}
	return node
}

// subgraphAttrs returns the attributes set directly on a subgraph
func subgraphAttrs(stmts []ast.Stmt) []*ast.Attr {
	var attrs []*ast.Attr
	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case *ast.Attr:
			attrs = append(attrs, stmt)
		case *ast.AttrStmt:
			if stmt.Kind == ast.GraphKind {
				attrs = append(attrs, stmt.Attrs...)
			}
		}
	}
	return attrs
}

func hasAttr(stmts []ast.Stmt, key, val string) bool {
	for _, stmt := range stmts {
		if attr, ok := stmt.(*ast.Attr); ok && attr.Key == key && attr.Val == val {
			return true
		}
	}
	return false
}

func (context *parserContext) parseNode(src *ast.NodeStmt) *layout.Node {
	node := context.ensureNode(src.Node.ID)
	applyNodeAttrs(node, src.Attrs)
	context.notePos(node, src.Attrs)
	return node
}

func (context *parserContext) parseEdge(edgeStmt *ast.EdgeStmt) {
	sources := context.ensureVertex(edgeStmt.From)
	sourcePort := vertexPort(edgeStmt.From)
	to := edgeStmt.To
	for to != nil {
		targets := context.ensureVertex(to.Vertex)
		targetPort := vertexPort(to.Vertex)
		for _, source := range sources {
			for _, target := range targets {
				edge := layout.NewEdge(source, target)

				edge.Directed = to.Directed
				edge.From = source
				edge.To = target
				edge.FromPort = sourcePort
				edge.ToPort = targetPort

				applyEdgeAttrs(edge, context.edgeAttrs)
				applyEdgeAttrs(edge, edgeStmt.Attrs)

				context.Graph.Edges = append(context.Graph.Edges, edge)
			}
		}

		sources = targets
		sourcePort = targetPort
		to = to.To
	}
}

// vertexPort returns the compass point of a node vertex, if any. Named
// ports (record fields) are not supported and ignored.
func vertexPort(v ast.Vertex) layout.Compass {
	node, ok := v.(*ast.Node)
	if !ok || node.Port == nil || node.Port.CompassPoint == ast.CompassPointNone {
		return layout.CompassAuto
	}
	return layout.Compass(node.Port.CompassPoint.String())
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
	var color layout.Color
	filled := false
	defer func() {
		// style=filled without fillcolor fills with the outline color
		if filled && node.FillColor == nil && color != nil {
			node.FillColor = color
		}
	}()
	for _, attr := range attrs {
		switch attr.Key {
		case "style":
			filled = strings.Contains(attr.Val, "filled")
			setLineStyle(&node.LineStyle, attr.Val)
		case "fixedsize":
			node.FixedSize = attr.Val == "true" || attr.Val == "shape"
		case "peripheries":
			if n, err := strconv.Atoi(attr.Val); err == nil {
				node.Peripheries = n
			}
		case "image":
			setString(&node.Image, attr.Val)
		case "shape":
			setShape(&node.Shape, attr.Val)
		case "label":
			setString(&node.Label, attr.Val)
		case "color":
			setColor(&color, attr.Val)
			setColor(&node.LineColor, attr.Val)
		case "fontcolor":
			setColor(&node.FontColor, attr.Val)
		case "fontname":
			setString(&node.FontName, attr.Val)
		case "fontsize":
			setLength(&node.FontSize, attr.Val, layout.Point)
		case "pencolor":
			setColor(&node.LineColor, attr.Val)
		case "penwidth":
			setLength(&node.LineWidth, attr.Val, layout.Point)
		case "fillcolor":
			setColor(&node.FillColor, attr.Val)
		case "width":
			setLength(&node.Radius.X, attr.Val, layout.Inch*0.5)
		case "height":
			setLength(&node.Radius.Y, attr.Val, layout.Inch*0.5)
		case "tooltip":
			setString(&node.Tooltip, attr.Val)
		case "pos":
			if p, ok := parsePoint(attr.Val); ok {
				node.Center = p
			}
		}
	}
}

func applyEdgeAttrs(edge *layout.Edge, attrs []*ast.Attr) {
	for _, attr := range attrs {
		switch attr.Key {
		case "weight":
			setFloat(&edge.Weight, attr.Val)
		case "style":
			setLineStyle(&edge.LineStyle, attr.Val)
		case "pos":
			edge.Path = parseSpline(attr.Val)
		case "lp":
			if p, ok := parsePoint(attr.Val); ok {
				edge.LabelPos = p
			}
		case "label":
			setString(&edge.Label, attr.Val)
		case "color":
			setColor(&edge.LineColor, attr.Val)
		case "fontcolor":
			setColor(&edge.FontColor, attr.Val)
		case "dir":
			switch fixstring(attr.Val) {
			case "back":
				edge.ArrowHead, edge.ArrowTail = layout.ArrowNone, layout.ArrowNormal
			case "both":
				edge.ArrowHead, edge.ArrowTail = layout.ArrowNormal, layout.ArrowNormal
			case "none":
				edge.ArrowHead, edge.ArrowTail = layout.ArrowNone, layout.ArrowNone
			case "forward":
				edge.ArrowHead, edge.ArrowTail = layout.ArrowNormal, layout.ArrowNone
			}
		case "arrowhead":
			edge.ArrowHead = layout.Arrow(fixstring(attr.Val))
		case "arrowtail":
			edge.ArrowTail = layout.Arrow(fixstring(attr.Val))
		case "headport":
			edge.ToPort = layout.Compass(fixstring(attr.Val))
		case "tailport":
			edge.FromPort = layout.Compass(fixstring(attr.Val))
		case "fontname":
			setString(&edge.FontName, attr.Val)
		case "fontsize":
			setLength(&edge.FontSize, attr.Val, layout.Point)
		case "pencolor":
			setColor(&edge.LineColor, attr.Val)
		case "penwidth":
			setLength(&edge.LineWidth, attr.Val, layout.Point)
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

// setLineStyle picks the stroke style out of a comma separated style list
func setLineStyle(t *layout.LineStyle, value string) {
	for s := range strings.SplitSeq(value, ",") {
		switch strings.TrimSpace(s) {
		case "dashed":
			*t = layout.Dashed
		case "dotted":
			*t = layout.Dotted
		case "bold":
			*t = layout.Bold
		}
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
	case "none":
		*t = layout.None
	case "record", "Mrecord":
		*t = layout.Record
	default:
		*t = layout.Auto
	}
}

func setString(t *string, value string) {
	*t = fixstring(value)
}

func fixstring(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return strings.Replace(s, "\\n", "\n", -1)
}
