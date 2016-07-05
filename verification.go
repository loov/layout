package layout

import (
	"bytes"
	"errors"
	"fmt"
)

func VerifyProperDigraph(graph *Graph) error {
	var buf bytes.Buffer
	for _, src := range graph.Nodes {
		for _, did := range src.Out {
			dst := graph.Nodes[did]
			if dst.Rank-src.Rank > 1 {
				fmt.Fprintf(&buf, "\t%v[%v] -> %v[%v]\n", src.ID, src.Rank, dst.ID, dst.Rank)
			}
		}
	}

	if buf.Len() > 0 {
		return errors.New("Invalid edges:\n" + buf.String())
	}
	return nil
}
