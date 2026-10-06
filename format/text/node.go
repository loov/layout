package text

import (
	"cmp"
	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
	"math"
	"slices"
)

// record draws field texts in their boxes, see placed, with dividers between
// sibling fields; row maps the top and bottom of fields to the rows of the
// borders and dividers around them
func (c *canvas) record(rec *draw.Record, origin layout.Vector, col, row func(layout.Length) int) {
	if len(rec.Fields) == 0 {
		x0, y0 := col(origin.X+layout.Length(rec.X0)), row(origin.Y+layout.Length(rec.Y0))
		x1, y1 := col(origin.X+layout.Length(rec.X1)), row(origin.Y+layout.Length(rec.Y1))
		lines := draw.Lines(rec.Text)
		for i, line := range lines {
			c.text(placed(x0, x1, line), (y0+y1)/2-(len(lines)-1)/2+i, line.Text)
		}
		return
	}
	// dividers first, so that field texts win when rows are too coarse
	for i, field := range rec.Fields {
		if i == 0 {
			continue
		}
		if rec.Vertical {
			y := row(origin.Y + layout.Length(field.Y0))
			for x := col(origin.X+layout.Length(field.X0)) + 1; x < col(origin.X+layout.Length(field.X1)); x++ {
				c.set(x, y, '─')
			}
		} else {
			x := col(origin.X + layout.Length(field.X0))
			for y := row(origin.Y+layout.Length(field.Y0)) + 1; y < row(origin.Y+layout.Length(field.Y1)); y++ {
				c.set(x, y, '│')
			}
		}
	}
	for _, field := range rec.Fields {
		c.record(field, origin, col, row)
	}
}

// layoutRecord computes the record fields of a node, measuring text as
// the layout did
func layoutRecord(graph *layout.Graph, node *layout.Node, box layout.NodeBox) *draw.Record {
	var lineWidth func(string) float64
	if graph.MeasureText != nil {
		lineWidth = func(line string) float64 { return float64(graph.MeasureText(line, node.FontName, box.FontSize)) }
	}
	sideways := graph.RankDir == layout.LeftToRight || graph.RankDir == layout.RightToLeft
	return draw.LayoutRecord(box.Label, sideways, float64(box.Size.X), float64(box.Size.Y), float64(graph.LineHeight), float64(box.FontSize), lineWidth)
}

// clusterBox returns the cells of the frame of the cluster at index i
func (c *canvas) clusterBox(i int) [4]int {
	cluster, box := c.l.Graph.Clusters[i], c.l.Clusters[i]
	x0, y0 := c.col(box.TopLeft.X), c.row(box.TopLeft.Y)
	x1, y1 := c.col(box.BottomRight.X), c.row(box.BottomRight.Y)
	if cluster.Label != "" {
		// the label fits, spaced, between the corners; layouts not made
		// for text can fall a few cells short
		x1 = max(x1, x0+clusterLabelWidth(cluster))
	}
	return [4]int{x0, y0, x1, y1}
}

// drawCluster draws the dashed frame and fill of the cluster at index i
func (c *canvas) drawCluster(i int) {
	cluster := c.l.Graph.Clusters[i]
	b := c.clusterBox(i)
	x0, y0, x1, y1 := b[0], b[1], b[2], b[3]
	c.ids++
	c.pen = pen{ink: c.color(rgb(cluster.LineColor)), dashed: true, edge: c.ids, frame: true}
	c.fill(x0, y0, x1, y1, c.color(rgb(cluster.FillColor)))
	c.frame(x0, y0, x1, y1)
}

// peripheryGap matches the layout's distance between a node's outlines
const peripheryGap = 4 * layout.Point

// nodeBox returns the cells of the box of a node, which holds its label
// with a space on either side; a dot takes one cell
func (c *canvas) nodeBox(node *layout.Node) [4]int {
	box := c.l.Node(node)
	if box.Shape == layout.PointShape {
		x, y := c.col(box.Center.X), c.row(box.Center.Y)
		return [4]int{x, y, x, y}
	}
	// a double outline takes the cells of a single one, so the room the
	// layout reserves for extra outlines stays outside the box
	inset := layout.Length(max(node.Peripheries-1, 0)) * peripheryGap
	x0, y0 := c.col(box.Left()+inset), c.row(box.Top()+inset)
	x1, y1 := c.col(box.Right()-inset), c.row(box.Bottom()-inset)
	label := draw.PlainLabel(box.Label)
	w, h := draw.LabelBox(label)
	// the box must hold the label and have distinct edges
	if box.Shape != layout.Record {
		x1 = max(x1, x0+w)
		if spare := x1 - x0 - 1 - draw.TextColumns(label); spare >= 3 && spare%2 == 1 {
			x0, x1 = c.evenLabel(node, x0, y0, x1, y1)
		}
	}
	x1 = max(x1, x0+2)
	y1 = max(y1, y0+2)
	if box.Shape == layout.Record {
		y1 = max(y1, y0+draw.RecordRows(layoutRecord(c.l.Graph, node, box))+1)
	} else {
		y1 = max(y1, y0+h)
	}
	if !c.sideways() {
		// a cell for every end along the top and bottom between the
		// corners, as the layout reserves, which rounding can fall a
		// cell short of
		top, bottom := c.sideEnds(node, y0, y1)
		x1 = max(x1, x0+max(len(top), len(bottom))+1)
	}
	return [4]int{x0, y0, x1, y1}
}

// sideEnd is where an edge ends along the top or bottom of a node, in
// cells, and whether it goes on straight past the first bend
type sideEnd struct {
	exact    float64
	straight bool
}

// sideEnds returns the ends of edges along the top and the bottom of a
// node whose box is on rows y0 to y1, and of edges that come straight
// down or up to it
func (c *canvas) sideEnds(node *layout.Node, y0, y1 int) (top, bottom []sideEnd) {
	add := func(path []layout.Vector) {
		p := path[0]
		// straight on past the first bend, see spreadSides
		straight := len(path) < 3 || absLength(path[2].X-path[1].X) < 0.01
		e := sideEnd{float64((p.X - c.origin.X) / c.cellW), straight}
		// an end straight down onto a rounder outline, which curves
		// below the top, ends on the top of the box too, see spreadSides
		vertical := len(path) > 1 && absLength(path[1].X-p.X) < 0.01
		switch r := c.row(p.Y); {
		case r <= y0 || vertical && path[1].Y < p.Y:
			top = append(top, e)
		case r >= y1 || vertical && path[1].Y > p.Y:
			bottom = append(bottom, e)
		}
	}
	for i, edge := range c.l.Graph.Edges {
		path := c.l.Edges[i].Path
		if len(path) == 0 {
			continue
		}
		if edge.From == node {
			add(path)
		}
		if edge.To == node {
			back := slices.Clone(path)
			slices.Reverse(back)
			add(back)
		}
	}
	return top, bottom
}

// evenLabel gives back one of an odd number of spare cells, which can't be
// split evenly around the label of node, on a side that no edge ends next
// to on the top or bottom, where spreadSides puts them; ends on the sides
// move onto them. It returns the new left and right columns of the box.
func (c *canvas) evenLabel(node *layout.Node, x0, y0, x1, y1 int) (int, int) {
	top, bottom := c.sideEnds(node, y0, y1)
	if max(len(top), len(bottom)) > x1-x0-2 {
		return x0, x1 // the ends need every cell between the corners
	}
	lo, hi := x1, x0 // the columns of the edge ends
	for _, ends := range [][]sideEnd{top, bottom} {
		slices.SortFunc(ends, func(a, b sideEnd) int { return cmp.Compare(a.exact, b.exact) })
		cells := make([]int, len(ends))
		exact, fixed := make([]float64, len(ends)), make([]bool, len(ends))
		for n, e := range ends {
			cells[n], exact[n], fixed[n] = int(e.exact+0.5), e.exact, e.straight
		}
		packEnds(cells, exact, fixed)
		for _, x := range cells {
			lo, hi = min(lo, x), max(hi, x)
		}
	}
	switch {
	case hi < x1-1:
		x1--
	case lo > x0+1:
		x0++
	}
	return x0, x1
}

// drawNode draws a node in its box, see nodeBox, and marks the box solid
// so that edges don't draw over it; an invisible node only keeps its box
// clear
func (c *canvas) drawNode(node *layout.Node) {
	graph := c.l.Graph
	box := c.l.Node(node)
	b := c.boxes[node]
	x0, y0, x1, y1 := b[0], b[1], b[2], b[3]
	if box.Shape == layout.PointShape {
		if node.Invisible {
			return
		}
		c.pen = pen{ink: c.color(rgb(node.LineColor))}
		if node.FillColor != nil {
			c.pen.ink = c.color(rgb(node.FillColor))
		}
		c.set(x0, y0, '●')
		if p := c.at(x0, y0); p != nil {
			p.solid, p.node = true, c.nodes[node]
		}
		return
	}
	c.pen = pen{ink: c.color(rgb(node.LineColor)), font: c.color(rgb(node.FontColor))}
	if !node.Invisible {
		c.fill(x0, y0, x1, y1, c.color(rgb(node.FillColor)))
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c.set(x, y, ' ')
			if p := c.at(x, y); p != nil {
				p.solid, p.node = true, c.nodes[node]
			}
			if x > x0 && x < x1 && y > y0 && y < y1 {
				c.hold(x, y) // blanks inside a box are part of it
			}
		}
	}
	if node.Invisible {
		return
	}
	// rounded corners for round shapes, Auto included as it is drawn as
	// an ellipse; there are no rounded double corners
	style := "┌┐└┘─│"
	switch {
	case box.Shape == layout.None:
		style = "      "
	case node.Peripheries > 1:
		style = "╔╗╚╝═║" // like double circles, of any shape
	case box.Shape == layout.Circle, box.Shape == layout.Ellipse, box.Shape == layout.Auto:
		style = "╭╮╰╯─│"
	}
	c.rect(x0, y0, x1, y1, style)
	if box.Shape == layout.Record {
		rec := layoutRecord(graph, node, box)
		c.evenRecord(rec, box.TopLeft())
		// the fields take whole rows, see fitRows, in place of their
		// share of the height
		fitRows(rec, y0, y1)
		row := func(v layout.Length) int { return int(math.Round(float64(v))) }
		c.record(rec, layout.Vector{X: box.TopLeft().X}, c.col, row)
		return
	}
	lines := draw.Lines(draw.PlainLabel(box.Label))
	top := (y0 + y1 + 1 - len(lines)) / 2
	if graph.PackEdgeEnds && c.sideways() {
		top = y0 + 1 // with the main path, along the first row
	}
	for i, line := range lines {
		c.text(placed(x0, x1, line), top+i, line.Text)
	}
}

// fitRows sets the tops and bottoms of the fields of rec to rows from y0
// to y1, the rows of the borders around it: fields stacked top to bottom
// take the rows their text needs and a row for the divider between them,
// see draw.RecordRows, and share the rows to spare in turn
func fitRows(rec *draw.Record, y0, y1 int) {
	rec.Y0, rec.Y1 = float64(y0), float64(y1)
	if !rec.Vertical {
		for _, field := range rec.Fields {
			fitRows(field, y0, y1)
		}
		return
	}
	n := len(rec.Fields)
	need := make([]int, n)
	spare := y1 - y0 - 1 - (n - 1)
	for i, field := range rec.Fields {
		need[i] = draw.RecordRows(field)
		spare -= need[i]
	}
	for i := 0; spare > 0 && n > 0; i = (i + 1) % n {
		need[i]++
		spare--
	}
	at := y0
	for i, field := range rec.Fields {
		fitRows(field, at, at+need[i]+1)
		at += need[i] + 1
	}
}

// evenRecord moves the divider after a field a column left when the
// field's text would have an odd number of spare columns around it, so
// that the text is centered; the field after takes the column.
func (c *canvas) evenRecord(rec *draw.Record, origin layout.Vector) {
	for i, field := range rec.Fields {
		if !rec.Vertical && i+1 < len(rec.Fields) && len(field.Fields) == 0 {
			x0, x1 := c.col(origin.X+layout.Length(field.X0)), c.col(origin.X+layout.Length(field.X1))
			if spare := x1 - x0 - 1 - draw.TextColumns(field.Text); spare >= 3 && spare%2 == 1 {
				x := field.X1 - float64(c.cellW)
				field.X1 = x
				setLeft(rec.Fields[i+1], x)
			}
		}
		c.evenRecord(field, origin)
	}
}

// setLeft moves the left edge of a field and of the fields along it
func setLeft(rec *draw.Record, x float64) {
	rec.X0 = x
	for i, field := range rec.Fields {
		if rec.Vertical || i == 0 {
			setLeft(field, x)
		}
	}
}
