package layout

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// CheckErrors does sanity check of content of graph
func (graph *Graph) CheckErrors() error {
	var errors []error
	for _, node := range graph.Nodes {
		countIn := NewNodeSet(graph.NodeCount())
		countOut := NewNodeSet(graph.NodeCount())

		for _, dst := range node.In {
			if int(dst.ID) >= graph.NodeCount() {
				errors = append(errors, fmt.Errorf("overflow in: %v -> %v", dst.ID, node.ID))
				continue
			}

			if !countIn.Include(dst) {
				errors = append(errors, fmt.Errorf("dup in: %v -> %v", dst.ID, node.ID))
			}
		}

		for _, dst := range node.Out {
			if int(dst.ID) >= graph.NodeCount() {
				errors = append(errors, fmt.Errorf("overflow out: %v -> %v", node.ID, dst.ID))
				continue
			}

			if !countOut.Include(dst) {
				errors = append(errors, fmt.Errorf("dup out: %v -> %v", node.ID, dst.ID))
			}
		}
	}

	// TODO: check for in/out cross-references
	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf("%v", errors)
}

// EdgeMatrixString creates a debug output showing both inbound and outbound edges
func (graph *Graph) EdgeMatrixString() string {
	lines := []string{}

	n := graph.NodeCount()
	stride := 2*n + 4
	table := make([]byte, n*stride)
	for i := range table {
		table[i] = ' '
	}

	createLine := func(nodes Nodes) string {
		line := make([]byte, n)
		for x := range line {
			line[x] = ' '
		}
		for _, node := range nodes {
			line[node.ID] = 'X'
		}
		return string(line)
	}

	formatEdges := func(node *Node) string {
		var b bytes.Buffer
		b.WriteString(strconv.Itoa(int(node.ID)))
		b.WriteString(": []int{")
		for i, dst := range node.Out {
			b.WriteString(strconv.Itoa(int(dst.ID)))
			if i+1 < len(node.Out) {
				b.WriteString(", ")
			}
		}
		b.WriteString("}")
		return b.String()
	}

	for _, node := range graph.Nodes {
		lines = append(lines, "|"+createLine(node.In)+"|"+createLine(node.Out)+"| "+formatEdges(node))
	}

	return strings.Join(lines, "\n")
}
