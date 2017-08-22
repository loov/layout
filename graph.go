package layout

import "fmt"

type NodeID int
type NodeIDs []NodeID

type Graph struct {
	Nodes     []*Node
	ByRank    []NodeIDs
	Positions []Vector
}

func NewGraph() *Graph {
	return &Graph{}
}

func NewGraphFrom(edgeList [][]int) *Graph {
	graph := NewGraph()

	for range edgeList {
		graph.Node()
	}

	for src, out := range edgeList {
		for _, dst := range out {
			graph.Edge(NodeID(src), NodeID(dst))
		}
	}

	return graph
}

type Node struct {
	ID      NodeID
	In      NodeIDs
	Out     NodeIDs
	Coef    float32
	GridX   float32
	Rank    int
	Virtual bool
}

type Vector struct{ X, Y float32 }

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

func (graph *Graph) CheckErrors() error {
	var errors []error
	for id, node := range graph.Nodes {
		countIn := make([]int, len(graph.Nodes))
		countOut := make([]int, len(graph.Nodes))

		for _, src := range node.In {
			if int(src) >= len(graph.Nodes) {
				errors = append(errors, fmt.Errorf("overflow in: %v -> %v", src, id))
				continue
			}
			countIn[src]++
			if countIn[src] > 1 {
				errors = append(errors, fmt.Errorf("dup in: %v -> %v", src, id))
			}
		}

		for _, dst := range node.Out {
			if int(dst) >= len(graph.Nodes) {
				errors = append(errors, fmt.Errorf("overflow out: %v -> %v", id, dst))
				continue
			}
			countOut[dst]++
			if countOut[dst] > 1 {
				errors = append(errors, fmt.Errorf("dup out: %v -> %v", id, dst))
			}
		}
	}

	// TODO: check for in/out cross-references

	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf("%v", errors)
}

func (graph *Graph) CrossingsUp(uid, vid NodeID) int {
	u, v := graph.Nodes[uid], graph.Nodes[vid]
	assert(u.Rank == v.Rank, "u and v are from different ranks")
	if u.Rank == 0 {
		return 0
	}

	count := 0
	prev := graph.ByRank[u.Rank-1]
	for _, w := range u.In {
		for _, z := range v.In {
			if prev.IndexOf(z) < prev.IndexOf(w) {
				count++
			}
		}
	}
	return count
}

func (graph *Graph) CrossingsDown(uid, vid NodeID) int {
	u, v := graph.Nodes[uid], graph.Nodes[vid]
	assert(u.Rank == v.Rank, "u and v are from different ranks")
	if u.Rank == len(graph.ByRank)-1 {
		return 0
	}

	count := 0
	next := graph.ByRank[u.Rank+1]
	for _, w := range u.In {
		for _, z := range v.In {
			if next.IndexOf(z) < next.IndexOf(w) {
				count++
			}
		}
	}
	return count
}

func (graph *Graph) Crossings(uid, vid NodeID) int {
	return graph.CrossingsDown(uid, vid) + graph.CrossingsUp(uid, vid)
}

func (graph *Graph) IsCyclic() bool {
	visited := make([]bool, len(graph.Nodes))
	recursing := make([]bool, len(graph.Nodes))

	var isCyclic func(n NodeID) bool
	isCyclic = func(n NodeID) bool {
		if visited[n] {
			return false
		}

		visited[n] = true
		recursing[n] = true
		for _, child := range graph.Nodes[n].Out {
			if !visited[child] && isCyclic(child) {
				return true
			}
			if recursing[child] {
				return true
			}
		}
		recursing[n] = false

		return false
	}

	for i := range graph.Nodes {
		if isCyclic(NodeID(i)) {
			return true
		}
	}

	return false
}

func (a NodeIDs) Copy() NodeIDs {
	r := make(NodeIDs, len(a))
	copy(r, a)
	return r
}

func (nodes *NodeIDs) Add(b NodeID) { *nodes = append(*nodes, b) }
func (nodes *NodeIDs) Include(b NodeID) {
	if !nodes.Contains(b) {
		nodes.Add(b)
	}
}

func (nodes *NodeIDs) Clear() { *nodes = nil }

func (nodes *NodeIDs) Remove(b NodeID) bool {
	i := nodes.IndexOf(b)
	if i >= 0 {
		*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
	}
	return i >= 0
}

func (nodes *NodeIDs) Contains(b NodeID) bool { return nodes.IndexOf(b) >= 0 }

func (nodes *NodeIDs) RemoveAll(b NodeID) {
	initial := *nodes
	*nodes = initial[:0]
	for _, node := range initial {
		if node != b {
			*nodes = append(*nodes, node)
		}
	}
}

func (nodes NodeIDs) IndexOf(b NodeID) int {
	for i, id := range nodes {
		if id == b {
			return i
		}
	}
	return -1
}
