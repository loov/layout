package glay

import (
	"fmt"
	"io"
)

type NodeID int
type NodeIDs []NodeID

func (a NodeIDs) Copy() NodeIDs {
	r := make(NodeIDs, len(a))
	copy(r, a)
	return r
}
func (nodes *NodeIDs) Add(b NodeID) { *nodes = append(*nodes, b) }
func (nodes *NodeIDs) Remove(b NodeID) {
	for i, nid := range *nodes {
		if nid == b {
			*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
			return
		}
	}
	panic("id not found")
}

type Graph struct {
	Nodes  []*Node
	ByRank []NodeIDs
}

func NewGraph() *Graph {
	return &Graph{}
}

type Node struct {
	ID      NodeID
	In      NodeIDs
	Out     NodeIDs
	Rank    int
	Virtual bool
}

func (graph *Graph) Node() (NodeID, *Node) {
	n := &Node{ID: NodeID(len(graph.Nodes))}
	graph.Nodes = append(graph.Nodes, n)
	return n.ID, n
}

func (graph *Graph) Edge(sid, did NodeID) {
	src, dst := graph.Nodes[sid], graph.Nodes[did]
	src.Out.Add(did)
	dst.In.Add(sid)
}

func (graph *Graph) WriteDOT(out io.Writer) (total int, err error) {
	write := func(format string, args ...interface{}) bool {
		var n int
		n, err = fmt.Fprintf(out, format, args...)
		total += n
		return err == nil
	}

	if !write("digraph G {\n") {
		return
	}

	for _, src := range graph.Nodes {
		if !src.Virtual {
			if !write("\t%v[rank = %v];\n", src.ID, src.Rank) {
				return
			}
		} else {
			if !write("\t%v[rank = %v; shape=circle];\n", src.ID, src.Rank) {
				return
			}
		}
		for _, did := range src.Out {
			if !write("\t%v -> %v;\n", src.ID, did) {
				return
			}
		}
	}

	if !write("}") {
		return
	}
	return
}

func (graph *Graph) WriteTGF(out io.Writer) (total int, err error) {
	write := func(format string, args ...interface{}) bool {
		var n int
		n, err = fmt.Fprintf(out, format, args...)
		total += n
		return err == nil
	}

	for _, src := range graph.Nodes {
		if !src.Virtual {
			if !write("%v %v\n", src.ID, src.ID) {
				return
			}
		} else {
			if !write("%v\n", src.ID) {
				return
			}
		}
	}

	if !write("#\n") {
		return
	}

	for _, src := range graph.Nodes {
		for _, did := range src.Out {
			if !write("%v %v\n", src.ID, did) {
				return
			}
		}
	}

	return
}
