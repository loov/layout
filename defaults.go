package layout

import (
	"github.com/loov/layout/internal/draw"
)

// AssignMissingValues fills in unset padding, font and size values on the
// graph, its nodes and edges from the graph defaults. Node sizes are
// estimated from their labels.
func (graph *lgraph) AssignMissingValues() {
	if graph.FontSize <= 0 {
		graph.FontSize = graph.LineHeight * 14 / 16
	}
	if graph.NodePadding <= 0 {
		graph.NodePadding = graph.LineHeight
	}
	if graph.RowPadding <= 0 {
		graph.RowPadding = graph.LineHeight
	}
	if graph.EdgePadding <= 0 {
		graph.EdgePadding = 6 * Point
	}

	for _, node := range graph.Nodes {
		if node.Shape == "" {
			node.Shape = graph.Shape
		}

		if node.FontSize <= 0 {
			node.FontSize = graph.FontSize
		}

		if node.Shape == PointShape && node.Radius.X <= 0 && node.Radius.Y <= 0 {
			node.Radius = Vector{pointRadius, pointRadius}
		}
		if node.Radius.X <= 0 {
			node.Radius.X = graph.LineHeight
		}
		if node.Radius.Y <= 0 {
			node.Radius.Y = graph.LineHeight
		}
		if !node.FixedSize && node.Shape != PointShape {
			labelRadius := graph.textRadius(node.DefaultLabel(), node.FontName, node.FontSize)
			if draw.IsHTMLLabel(node.DefaultLabel()) {
				labelRadius = graph.htmlLabelRadius(node.DefaultLabel(), node.FontName, node.FontSize)
			}
			labelRadius.X += node.FontSize * 0.5
			labelRadius.Y += node.FontSize * 0.25
			if node.Shape == Record {
				w, h := draw.RecordSize(node.DefaultLabel(), float64(graph.LineHeight), float64(node.FontSize), graph.lineWidth(node.FontName, node.FontSize))
				labelRadius = Vector{Length(w) / 2, Length(h) / 2}
			}

			if node.Radius.X < labelRadius.X {
				node.Radius.X = labelRadius.X
			}
			if node.Radius.Y < labelRadius.Y {
				node.Radius.Y = labelRadius.Y
			}
		}
		if node.Shape == Circle || node.Shape == Square || node.Shape == PointShape {
			// drawn with the larger radius on both axes
			r := max(node.Radius.X, node.Radius.Y)
			node.Radius = Vector{r, r}
		}
		if node.Peripheries > 1 {
			extra := Length(node.Peripheries-1) * peripheryGap
			node.pad = Vector{extra, extra}
			node.Radius = node.Radius.Add(node.pad)
		}
	}

	for _, edge := range graph.Edges {
		if edge.Weight < epsilon {
			edge.Weight = epsilon
		}
		if edge.MinLen < 1 {
			edge.MinLen = 1
		}
		if edge.FontSize <= 0 {
			edge.FontSize = graph.FontSize
		}
		if draw.IsHTMLLabel(edge.Label) {
			edge.LabelRadius = graph.htmlLabelRadius(edge.Label, edge.FontName, edge.FontSize)
		} else if edge.Label != "" {
			edge.LabelRadius = graph.textRadius(edge.Label, edge.FontName, edge.FontSize)
		}
	}
}
