package layout

import (
	"slices"

	"github.com/loov/layout/internal/draw"
)

// prepareText sets ortho edges and spacing that leaves rows between ranks
// for horizontal runs and arrowheads, and room for labels; channels with
// more runs than fit stretch, see orthoEdges.
// Sideways, it packs edge ends along the top of nodes, see PackEdgeEnds;
// otherwise it makes nodes with self-loops tall enough for the loop ends.
func (graph *lgraph) prepareText() {
	graph.Splines = SplinesOrtho
	if graph.LineHeight <= 0 {
		graph.LineHeight = 16
	}
	rows := Length(2)
	if slices.ContainsFunc(graph.Edges, func(edge *ledge) bool { return edge.Label != "" }) {
		rows += 2
	}
	graph.RowPadding = graph.LineHeight * rows
	graph.NodePadding = graph.LineHeight * 2
	graph.EdgePadding = graph.LineHeight

	// text draws the dividers of records on rows of their own, and boxes
	// as wide as their labels take cells, which estimates can fall short of
	if graph.FontSize <= 0 {
		graph.FontSize = graph.LineHeight * 14 / 16
	}
	cellW := graph.cellWidth()

	// text draws every label in cells of one size, so edge and cluster
	// labels, records and HTML tables are measured that way too
	graph.MeasureText = func(line, _ string, _ Length) Length {
		return Length(draw.Columns(line)) * cellW
	}
	for _, node := range graph.Nodes {
		// resolved here, so that the checks below see the default too
		if node.Shape == Auto {
			node.Shape = graph.Shape
		}
		if node.Shape == PointShape {
			continue
		}
		tableRecord(graph, node)
		if node.Shape == None && node.Radius.X <= 0 && node.Radius.Y <= 0 && !node.FixedSize {
			// text draws the label alone, in a frame of blank cells where
			// edges end, see draw.PlainBox; no more room than that
			w, h := draw.PlainBox(draw.PlainLabel(node.DefaultLabel()))
			node.Radius = Vector{Length(w) * cellW / 2, graph.LineHeight * Length(h) / 2}
			node.FixedSize = true
			continue
		}
		if node.Radius.X <= 0 {
			node.Radius.X = graph.LineHeight // the layout's default
		}
		switch node.Shape {
		case Record:
			rows := draw.RecordRows(draw.ParseRecord(node.DefaultLabel(), graph.RankDir.Sideways()))
			node.Radius.Y = max(node.Radius.Y, graph.LineHeight*Length(rows+1)/2)
			graph.reserveRecord(node, cellW)
		case Diamond:
			// the cells text draws it in, see draw.DiamondSize, which hold
			// its label
		default:
			// the label's rows and a border row above and below
			w, h := draw.LabelBox(draw.PlainLabel(node.DefaultLabel()))
			node.Radius.X = max(node.Radius.X, Length(w)*cellW/2)
			node.Radius.Y = max(node.Radius.Y, graph.LineHeight*Length(h)/2)
		}
	}

	// sideways, the main path runs along the top row of the nodes, with
	// further edges a row each below it; the layout makes room for them
	sideways := graph.RankDir.Sideways()
	graph.PackEdgeEnds = sideways
	if sideways {
		// nodes in a rank are stacked down the rows, which are twice as
		// tall as columns are wide; keep the same visual spacing
		graph.NodePadding = graph.LineHeight / 2
		// packing makes nodes taller; text draws circles as boxes either
		// way, and a circle would widen as much
		for _, node := range graph.Nodes {
			if node.Shape == Circle {
				node.Shape = Ellipse
			}
		}
		return
	}

	// self-loops stack down the right side, each leaving and returning a
	// quarter of its share from the share's ends: four rows a loop put
	// them a row above and below its middle
	loops := map[*lnode]int{}
	for _, edge := range graph.Edges {
		if edge.loop() {
			loops[edge.From]++
		}
	}
	for node, n := range loops {
		node.Radius.Y = max(node.Radius.Y, 2*Length(n)*graph.LineHeight)
		if node.Shape == Circle {
			// drawn as a box either way; a circle would widen as much
			node.Shape = Ellipse
		}
	}
}

// reserveRecord widens a record node until each field holds its text
// between the dividers. Fields share the width beyond their estimated
// sizes evenly, so a field grows by the record's growth divided by the
// fields beside it at each level.
func (graph *lgraph) reserveRecord(node *lnode, cellW Length) {
	if node.FontSize <= 0 {
		// as the layout sizes the fields
		node.FontSize = graph.FontSize
		defer func() { node.FontSize = 0 }()
	}
	grow := Length(0)
	var walk func(rec *draw.Record, share Length)
	walk = func(rec *draw.Record, share Length) {
		if rec.Leaf() {
			w, _ := draw.LabelBox(rec.Text)
			need := Length(w) * cellW
			grow = max(grow, (need-Length(rec.Width()))*share)
			return
		}
		if !rec.Vertical {
			share *= Length(len(rec.Fields))
		}
		for _, field := range rec.Fields {
			walk(field, share)
		}
	}

	walk(draw.LayoutRecord(node.DefaultLabel(), graph.RankDir.Sideways(), 2*float64(node.Radius.X), 2*float64(node.Radius.Y),
		float64(graph.LineHeight), float64(node.FontSize), graph.lineWidth(node.FontName, node.FontSize)), 1)
	node.Radius.X += grow / 2
}

// cellWidth returns the width of a character cell of format/text, which
// draws a cell FontSize*0.55 wide
func (graph *lgraph) cellWidth() Length { return graph.FontSize * 0.55 }

// cell returns a character cell of format/text in the frame where ranks
// run down: X along the ranks, a column or a line sideways, and Y across
// them, a line or a column sideways
func (graph *lgraph) cell() Vector {
	if graph.RankDir.Sideways() {
		return Vector{X: graph.LineHeight, Y: graph.cellWidth()}
	}
	return Vector{X: graph.cellWidth(), Y: graph.LineHeight}
}
