// Package dot parses the Graphviz dot file format into layout graphs and
// writes laid out graphs back as dot.
//
// The parser reads these attributes; everything else is ignored without
// an error.
//
//   - graph: rankdir, splines (polyline, line, ortho), nodesep, ranksep,
//     bb
//   - subgraph: rank (same, min, source, max, sink); for cluster
//     subgraphs also label, color, pencolor, fillcolor, bgcolor and
//     style (filled, invis)
//   - node: label, shape, style, color, pencolor, fillcolor, fontcolor,
//     fontname, fontsize, penwidth, width, height, fixedsize,
//     peripheries, image, tooltip, pos
//   - edge: label, style, color, pencolor, fontcolor, fontname, fontsize,
//     penwidth, weight, minlen, dir, arrowhead, arrowtail, headport,
//     tailport, tooltip, pos, lp
//
// When every node has a pos, the positions go into layout.Node.Pos and
// layout keeps them, like dot -n.
//
// Known gaps:
//
//   - Shapes other than box, rect, rectangle, square, circle,
//     doublecircle, ellipse, oval, none, plaintext, plain, point, record
//     and Mrecord use the graph default; that includes diamond. Mrecord
//     draws with square corners.
//   - Arrowheads other than normal, vee, dot, odot and none draw as a
//     normal arrowhead.
//   - Colors are names from the X11 scheme or #RRGGBB and #RRGGBBAA;
//     HSV values, color lists and gradients are ignored.
//   - Styles other than solid, dashed, dotted, bold, filled and invis
//     are ignored; rounded and diagonals have no effect.
//   - Ports name compass points only; a record field port such as
//     "node:f0" attaches to the node as a whole.
//   - minlen=0 is ignored; put such nodes in a rank=same subgraph.
//   - The graph label, labels on subgraphs that are not clusters,
//     xlabel, headlabel and taillabel are not drawn.
//   - concentrate shares the starts of ortho edges, see
//     layout.Graph.MergeEdges; edges into a node stay apart.
//   - Layout controls such as constraint, group, ordering, compound with
//     lhead and ltail, newrank, size, ratio and rotate have no effect.
package dot

import (
	"errors"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"

	"gonum.org/v1/gonum/graph/formats/dot"
	"gonum.org/v1/gonum/graph/formats/dot/ast"
)

// Parse reads dot from r and returns every graph it contains.
func Parse(r io.Reader) ([]*layout.Graph, error) { return parse(dot.Parse(r)) }

// ParseFile reads dot from the file at path and returns every graph it contains.
func ParseFile(path string) ([]*layout.Graph, error) { return parse(dot.ParseFile(path)) }

// ParseString parses dot from s and returns every graph it contains.
func ParseString(s string) ([]*layout.Graph, error) { return parse(dot.ParseString(s)) }

// parseError is panicked by the parser on input it cannot represent and
// returned by parse
type parseError struct{ error }

// parse converts a parsed dot file into layout graphs
func parse(file *ast.File, err error) (_ []*layout.Graph, failed error) {
	if err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			perr, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			failed = perr.error
		}
	}()

	graphs := []*layout.Graph{}
	for _, graphStmt := range file.Graphs {
		graphStmt.ID = unquote(graphStmt.ID)
		literal := map[string]bool{}
		unquoteStmts(graphStmt.Stmts, literal)
		parser := &parserContext{positioned: map[*layout.Node]bool{}, outlines: map[*layout.Node]*outlines{}, literal: literal}
		parser.Graph = layout.NewGraph()
		if graphStmt.Strict {
			parser.strict = map[[2]*layout.Node]*strictEdge{}
		}
		parser.parse(graphStmt)
		graphs = append(graphs, parser.Graph)
	}

	return graphs, nil
}

// unquoteStmts strips dot quotes from every identifier and attribute
// value, so "a" and a name the same node and shape="box" reads as box.
// Labels keep their quotes and escapes for expandLabel. Quoted node ids
// that read as HTML, such as "<init>", go into literal, so that their
// default label stays text.
func unquoteStmts(stmts []ast.Stmt, literal map[string]bool) {
	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case *ast.NodeStmt:
			stmt.Node.ID = unquoteID(stmt.Node.ID, literal)
			unquoteAttrs(stmt.Attrs)
		case *ast.EdgeStmt:
			unquoteVertex(stmt.From, literal)
			for to := stmt.To; to != nil; to = to.To {
				unquoteVertex(to.Vertex, literal)
			}
			unquoteAttrs(stmt.Attrs)
		case *ast.AttrStmt:
			unquoteAttrs(stmt.Attrs)
		case *ast.Attr:
			unquoteAttrs([]*ast.Attr{stmt})
		case *ast.Subgraph:
			stmt.ID = unquote(stmt.ID)
			unquoteStmts(stmt.Stmts, literal)
		}
	}
}

// unquoteID unquotes a node id, noting quoted ids that read as HTML
func unquoteID(id string, literal map[string]bool) string {
	unquoted := unquote(id)
	if unquoted != id && draw.IsHTMLLabel(unquoted) {
		literal[unquoted] = true
	}
	return unquoted
}

func unquoteVertex(v ast.Vertex, literal map[string]bool) {
	switch v := v.(type) {
	case *ast.Node:
		v.ID = unquoteID(v.ID, literal)
	case *ast.Subgraph:
		v.ID = unquote(v.ID)
		unquoteStmts(v.Stmts, literal)
	}
}

func unquoteAttrs(attrs []*ast.Attr) {
	for _, attr := range attrs {
		if attr.Key != "label" {
			attr.Val = unquote(attr.Val)
		}
	}
}

// unquote strips the quotes of a dot string and undoes the escapes that
// quote in write.go adds.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return unescape.Replace(s[1 : len(s)-1])
	}
	return s
}

var unescape = strings.NewReplacer(`\\`, `\`, `\"`, `"`, `\n`, "\n")

// expandLabel interprets a label value. In quoted strings \n, \l and \r
// break lines and names replace their escapes, given as pairs such as
// `\N`, node.ID; other escapes are kept for record labels. A final line
// break only ends the last line, as in Graphviz. A quoted label that
// reads as HTML, such as "<init>", gets the literalMark prefix. HTML
// labels and plain ids are returned as they are.
func expandLabel(raw string, names ...string) string {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return raw
	}
	escapes := append([]string{`\\`, `\`, `\"`, `"`, `\n`, "\n", `\l`, "\n", `\r`, "\n"}, names...)
	label := strings.TrimSuffix(strings.NewReplacer(escapes...).Replace(raw[1:len(raw)-1]), "\n")
	if draw.IsHTMLLabel(label) {
		label = literalMark + label
	}
	return label
}

// literalMark is a zero width space that keeps quoted text in angle
// brackets from being drawn as an HTML label. It is not visible and
// takes no room.
const literalMark = "\u200b"

// parserContext holds the attribute defaults in effect for a (sub)graph
type parserContext struct {
	Graph   *layout.Graph
	Cluster string

	allAttrs  []*ast.Attr
	nodeAttrs []*ast.Attr
	edgeAttrs []*ast.Attr

	touched []*layout.Node // nodes referenced in this (sub)graph

	positioned map[*layout.Node]bool // nodes with a pos attribute
	literal    map[string]bool       // quoted node ids that read as HTML
	outlines   map[*layout.Node]*outlines

	// strict holds the edges of a strict graph by their ends; nil when
	// the graph allows multi-edges
	strict map[[2]*layout.Node]*strictEdge
}

// strictEdge is an edge of a strict graph with every attribute assigned
// to it so far, as later statements for the same ends merge into it
type strictEdge struct {
	edge  *layout.Edge
	attrs []*ast.Attr
}

// outlines is what decides a node's peripheries over all of its
// attribute assignments: Graphviz draws doublecircle with two unless
// peripheries is set
type outlines struct {
	double bool // the shape is doublecircle
	set    bool // peripheries was set explicitly
}

func (context *parserContext) parse(src *ast.Graph) {
	context.Graph.ID = src.ID
	context.Graph.Directed = src.Directed
	context.parseStmts(src.Stmts)
	for _, node := range context.Graph.Nodes {
		// record labels are never HTML, and their <port> must lead
		if node.Shape == layout.Record {
			node.Label = strings.TrimPrefix(node.Label, literalMark)
		}
	}
	// a port that names a field of a record, such as "n", is that field
	// and not a compass point; the edge attaches to the node as a whole
	field := func(node *layout.Node, port layout.Compass) bool {
		return node.Shape == layout.Record && strings.Contains(node.Label, "<"+string(port)+">")
	}
	for _, edge := range context.Graph.Edges {
		if field(edge.From, edge.FromPort) {
			edge.FromPort = layout.CompassAuto
		}
		if field(edge.To, edge.ToPort) {
			edge.ToPort = layout.CompassAuto
		}
	}
	applyGraphAttrs(context.Graph, context.allAttrs)
	context.pin()
}

// pin flips the y axis of the positions from dot's upwards to ours when
// every node has a pos; layout then keeps them.
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
		top = max(top, node.Pos.Y+node.MinSize.Y/2)
	}
	// the bounding box, when present, gives the exact extent; mirroring
	// within it keeps coordinates in place
	for _, attr := range context.allAttrs {
		if attr.Key == "bb" {
			if corners := strings.Split(attr.Val, ","); len(corners) == 4 {
				bottom, okb := parseFloat(corners[1], layout.Point)
				h, okh := parseFloat(corners[3], layout.Point)
				if okb && okh {
					top = layout.Length(bottom + h)
				}
			}
		}
	}
	flip := func(v *layout.Vector) { v.Y = top - v.Y }
	for _, node := range graph.Nodes {
		flip(node.Pos)
	}
	for _, edge := range graph.Edges {
		for i := range edge.Pos {
			flip(&edge.Pos[i])
		}
		if edge.LabelPos != nil {
			flip(edge.LabelPos)
		}
	}
}

// notePos remembers nodes that got a valid pos attribute
func (context *parserContext) notePos(node *layout.Node, attrs []*ast.Attr) {
	for _, attr := range attrs {
		if _, ok := parsePoint(attr.Val); ok && attr.Key == "pos" {
			context.positioned[node] = true
		}
	}
}

// parsePoint reads "x,y" in points, ignoring a trailing "!"
func parsePoint(s string) (layout.Vector, bool) {
	x, y, ok := strings.Cut(strings.TrimSuffix(s, "!"), ",")
	if !ok {
		return layout.Vector{}, false
	}
	fx, okx := parseFloat(x, layout.Point)
	fy, oky := parseFloat(y, layout.Point)
	return layout.Vector{X: layout.Length(fx), Y: layout.Length(fy)}, okx && oky
}

// parseSpline reads an edge pos: points separated by spaces, where "s,x,y"
// and "e,x,y" are the arrow tips at the start and end
func parseSpline(s string) []layout.Vector {
	var start, end *layout.Vector
	var points []layout.Vector
	for field := range strings.FieldsSeq(s) {
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
	// Repeated controls encode straight segments in our DOT output.
	points = slices.Compact(points)
	// The spline stops at the arrow base; replace the base with the tip,
	// which lies on the line continuing the spline.
	if start != nil {
		if len(points) > 1 {
			points = points[1:]
		}
		points = append([]layout.Vector{*start}, points...)
	}
	if end != nil {
		if len(points) > 1 {
			points = points[:len(points)-1]
		}
		points = append(points, *end)
	}
	return slices.Compact(points)
}

// applyGraphAttrs applies graph level attributes
func applyGraphAttrs(graph *layout.Graph, attrs []*ast.Attr) {
	for _, attr := range attrs {
		switch attr.Key {
		case "rankdir":
			switch strings.ToUpper(attr.Val) {
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
			switch strings.ToLower(attr.Val) {
			case "polyline":
				graph.Splines = layout.SplinesPolyline
			case "line", "false":
				graph.Splines = layout.SplinesLine
			case "ortho":
				graph.Splines = layout.SplinesOrtho
			}
		case "concentrate":
			graph.MergeEdges, _ = strconv.ParseBool(attr.Val)
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
			context.parseSubgraph(stmt)
		}
	}
}

// parseSubgraph parses a subgraph in a context that inherits the current
// attribute defaults, recording clusters and rank constraints. Its nodes
// count as touched in this context too.
func (context *parserContext) parseSubgraph(src *ast.Subgraph) *parserContext {
	start := len(context.Graph.Clusters)
	subcontext := &parserContext{positioned: context.positioned, outlines: context.outlines, strict: context.strict, literal: context.literal}
	subcontext.Graph = context.Graph
	subcontext.allAttrs = append(subcontext.allAttrs, context.allAttrs...)
	subcontext.nodeAttrs = append(subcontext.nodeAttrs, context.nodeAttrs...)
	subcontext.edgeAttrs = append(subcontext.edgeAttrs, context.edgeAttrs...)
	subcontext.parseStmts(src.Stmts)
	for _, node := range subcontext.touched {
		if !slices.Contains(context.touched, node) {
			context.touched = append(context.touched, node)
		}
	}
	if strings.HasPrefix(src.ID, "cluster") && len(subcontext.touched) > 0 {
		cluster := &layout.Cluster{ID: src.ID, Nodes: subcontext.touched}
		// outer before inner, so inner clusters are drawn on top
		for _, inner := range context.Graph.Clusters[start:] {
			if inner.Parent == nil {
				inner.Parent = cluster
			}
		}
		context.Graph.Clusters = slices.Insert(context.Graph.Clusters, start, cluster)
		var color layout.Color
		filled := false
		for _, attr := range subgraphAttrs(src.Stmts) {
			switch attr.Key {
			case "label":
				setString(&cluster.Label, expandLabel(attr.Val, `\G`, src.ID))
			case "color":
				setColor(&color, attr.Val)
				setColor(&cluster.LineColor, attr.Val)
			case "pencolor":
				setColor(&cluster.LineColor, attr.Val)
			case "fillcolor", "bgcolor":
				setColor(&cluster.FillColor, attr.Val)
			case "style":
				filled = strings.Contains(attr.Val, "filled")
				cluster.Invisible = hasStyle(attr.Val, "invis")
			}
		}
		if filled && cluster.FillColor == nil {
			cluster.FillColor = color
		}
	}
	// rank set on an enclosing graph before this subgraph is inherited
	switch lastAttr(subcontext.allAttrs, "rank") {
	case "same":
		if len(subcontext.touched) > 1 {
			context.Graph.SameRank = append(context.Graph.SameRank, subcontext.touched)
		}
	case "min", "source":
		context.Graph.MinRank = appendMissing(context.Graph.MinRank, subcontext.touched)
	case "max", "sink":
		context.Graph.MaxRank = appendMissing(context.Graph.MaxRank, subcontext.touched)
	}
	return subcontext
}

func (context *parserContext) ensureNode(id string) *layout.Node {
	if id == "" {
		panic(parseError{errors.New("dot: empty node id")})
	}
	node, exists := context.Graph.NodeByID[id]
	if !exists {
		node = context.Graph.Node(id)
		node.Label = node.ID // dot's default label is the id, label="" is empty
		if context.literal[id] {
			node.Label = literalMark + id
		}
		context.outlines[node] = &outlines{}
		applyNodeAttrs(context.Graph.ID, node, context.nodeAttrs, context.outlines[node])
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

// appendMissing appends the nodes that are not yet in list, so that nested
// subgraphs inheriting a rank do not repeat their nodes.
func appendMissing(list, nodes []*layout.Node) []*layout.Node {
	for _, node := range nodes {
		if !slices.Contains(list, node) {
			list = append(list, node)
		}
	}
	return list
}

// lastAttr returns the value of the last assignment to key, since later
// assignments override earlier ones in Graphviz.
func lastAttr(attrs []*ast.Attr, key string) string {
	val := ""
	for _, attr := range attrs {
		if attr.Key == key {
			val = attr.Val
		}
	}
	return val
}

func (context *parserContext) parseNode(src *ast.NodeStmt) *layout.Node {
	node := context.ensureNode(src.Node.ID)
	applyNodeAttrs(context.Graph.ID, node, src.Attrs, context.outlines[node])
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
				if context.mergeStrict(source, target, sourcePort, targetPort, edgeStmt.Attrs) {
					continue
				}
				edge := layout.NewEdge(source, target)

				edge.Directed = to.Directed
				edge.From = source
				edge.To = target
				edge.FromPort = sourcePort
				edge.ToPort = targetPort

				attrs := slices.Concat(context.edgeAttrs, edgeStmt.Attrs)
				applyEdgeAttrs(context.Graph.ID, edge, attrs)
				if context.strict != nil {
					context.strict[[2]*layout.Node{source, target}] = &strictEdge{edge, attrs}
				}

				context.Graph.Edges = append(context.Graph.Edges, edge)
			}
		}

		sources = targets
		sourcePort = targetPort
		to = to.To
	}
}

// mergeStrict merges an edge statement between source and target into
// the existing edge between them in a strict graph, in either direction
// when undirected. It reports whether there was one. As in Graphviz, the
// statement's attributes apply but defaults set since do not.
func (context *parserContext) mergeStrict(source, target *layout.Node, sourcePort, targetPort layout.Compass, attrs []*ast.Attr) bool {
	prev, ok := context.strict[[2]*layout.Node{source, target}]
	if !ok && !context.Graph.Directed {
		prev, ok = context.strict[[2]*layout.Node{target, source}]
		sourcePort, targetPort = targetPort, sourcePort
	}
	if !ok {
		return false
	}
	if sourcePort != layout.CompassAuto {
		prev.edge.FromPort = sourcePort
	}
	if targetPort != layout.CompassAuto {
		prev.edge.ToPort = targetPort
	}
	prev.attrs = append(prev.attrs, attrs...)
	applyEdgeAttrs(context.Graph.ID, prev.edge, prev.attrs)
	return true
}

// vertexPort returns the compass point of a node vertex, if any. Named
// ports (record fields) are not supported and ignored.
func vertexPort(v ast.Vertex) layout.Compass {
	node, ok := v.(*ast.Node)
	if !ok || node.Port == nil || node.Port.CompassPoint == ast.CompassPointNone {
		return layout.CompassAuto
	}
	return parseCompass(node.Port.CompassPoint.String())
}

// parseCompass reads a port such as "n" or "port:n" as its compass point.
// Named ports and "_" give CompassAuto.
func parseCompass(port string) layout.Compass {
	if i := strings.LastIndexByte(port, ':'); i >= 0 {
		port = port[i+1:]
	}
	switch c := layout.Compass(port); c {
	case layout.North, layout.NorthEast, layout.East, layout.SouthEast,
		layout.South, layout.SouthWest, layout.West, layout.NorthWest, layout.Center:
		return c
	}
	return layout.CompassAuto
}

func (context *parserContext) ensureVertex(src ast.Vertex) []*layout.Node {
	switch src := src.(type) {
	case *ast.Node:
		return []*layout.Node{context.ensureNode(src.ID)}
	case *ast.Subgraph:
		return context.parseSubgraph(src).touched
	default:
		return nil
	}
}

// applyNodeAttrs applies attrs to node of the graph named graphID;
// outlines carries what earlier assignments to the node set
func applyNodeAttrs(graphID string, node *layout.Node, attrs []*ast.Attr, outlines *outlines) {
	var color layout.Color
	filled := false
	defer func() {
		// style=filled without fillcolor fills with the outline color
		if filled && node.FillColor == nil && color != nil {
			node.FillColor = color
		}
		// doublecircle is a circle with two outlines unless set otherwise
		if !outlines.set {
			node.Peripheries = 0
			if outlines.double {
				node.Peripheries = 2
			}
		}
	}()
	for _, attr := range attrs {
		switch attr.Key {
		case "style":
			filled = strings.Contains(attr.Val, "filled")
			node.Invisible = hasStyle(attr.Val, "invis")
			setLineStyle(&node.LineStyle, attr.Val)
		case "fixedsize":
			node.FixedSize = attr.Val == "true" || attr.Val == "shape"
		case "peripheries":
			if n, err := strconv.Atoi(attr.Val); err == nil {
				node.Peripheries, outlines.set = n, true
			}
		case "image":
			setString(&node.Image, attr.Val)
		case "shape":
			setShape(&node.Shape, attr.Val)
			outlines.double = attr.Val == "doublecircle"
		case "label":
			setString(&node.Label, expandLabel(attr.Val, `\N`, node.ID, `\G`, graphID))
			node.NoLabel = node.Label == ""
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
			setLength(&node.MinSize.X, attr.Val, layout.Inch)
		case "height":
			setLength(&node.MinSize.Y, attr.Val, layout.Inch)
		case "tooltip":
			setString(&node.Tooltip, attr.Val)
		case "pos":
			if p, ok := parsePoint(attr.Val); ok {
				node.Pos = &p
			}
		}
	}
}

// maxMinLen bounds minlen, since every rank an edge spans adds virtual
// nodes to a hierarchical layout
const maxMinLen = 1000

// applyEdgeAttrs applies all attributes of an edge of the graph named
// graphID, defaults first. Explicit arrowhead and arrowtail win over the
// arrows dir implies, wherever they appear.
func applyEdgeAttrs(graphID string, edge *layout.Edge, attrs []*ast.Attr) {
	var head, tail layout.Arrow
	defer func() {
		switch lastAttr(attrs, "dir") {
		case "back":
			edge.ArrowHead, edge.ArrowTail = layout.ArrowNone, layout.ArrowNormal
		case "both":
			edge.ArrowHead, edge.ArrowTail = layout.ArrowNormal, layout.ArrowNormal
		case "none":
			edge.ArrowHead, edge.ArrowTail = layout.ArrowNone, layout.ArrowNone
		case "forward":
			edge.ArrowHead, edge.ArrowTail = layout.ArrowNormal, layout.ArrowNone
		}
		if head != "" {
			edge.ArrowHead = head
		}
		if tail != "" {
			edge.ArrowTail = tail
		}
	}()
	for _, attr := range attrs {
		switch attr.Key {
		case "weight":
			setFloat(&edge.Weight, attr.Val)
		case "minlen":
			// minlen=0, meaning "same rank", is not supported; use rank=same
			if n, err := strconv.Atoi(attr.Val); err == nil && n > 0 {
				edge.MinLen = min(n, maxMinLen)
			}
		case "style":
			edge.Invisible = hasStyle(attr.Val, "invis")
			setLineStyle(&edge.LineStyle, attr.Val)
		case "pos":
			edge.Pos = parseSpline(attr.Val)
		case "lp":
			if p, ok := parsePoint(attr.Val); ok {
				edge.LabelPos = &p
			}
		case "label":
			name := edge.From.ID + "--" + edge.To.ID
			if edge.Directed {
				name = edge.From.ID + "->" + edge.To.ID
			}
			setString(&edge.Label, expandLabel(attr.Val, `\E`, name, `\T`, edge.From.ID, `\H`, edge.To.ID, `\G`, graphID))
		case "color":
			setColor(&edge.LineColor, attr.Val)
		case "fontcolor":
			setColor(&edge.FontColor, attr.Val)
		case "arrowhead":
			head = layout.Arrow(attr.Val)
		case "arrowtail":
			tail = layout.Arrow(attr.Val)
		case "headport":
			edge.ToPort = parseCompass(attr.Val)
		case "tailport":
			edge.FromPort = parseCompass(attr.Val)
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
	if color, ok := layout.ParseColor(value); ok {
		*t = color
	}
}

// parseFloat parses a number and scales it by unit; NaN and values that
// are infinite once scaled are rejected like unparsable ones
func parseFloat(value string, unit layout.Length) (float64, bool) {
	v, err := strconv.ParseFloat(value, 64)
	v *= float64(unit)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func setFloat(t *float64, value string) {
	if v, ok := parseFloat(value, 1); ok {
		*t = v
	}
}

func setLength(t *layout.Length, value string, unit layout.Length) {
	if v, ok := parseFloat(value, unit); ok {
		*t = layout.Length(v)
	}
}

// hasStyle reports whether the comma separated style list contains name
func hasStyle(value, name string) bool {
	for s := range strings.SplitSeq(value, ",") {
		if strings.TrimSpace(s) == name {
			return true
		}
	}
	return false
}

// setLineStyle picks the stroke style out of a comma separated style list
func setLineStyle(t *layout.LineStyle, value string) {
	for s := range strings.SplitSeq(value, ",") {
		switch strings.TrimSpace(s) {
		case "solid":
			*t = layout.Solid
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
	case "circle", "doublecircle":
		*t = layout.Circle
	case "ellipse", "oval":
		*t = layout.Ellipse
	case "none", "plaintext", "plain":
		*t = layout.None
	case "point":
		*t = layout.PointShape
	case "record", "Mrecord":
		*t = layout.Record
	default:
		*t = layout.Auto
	}
}

func setString(t *string, value string) {
	*t = value
}
