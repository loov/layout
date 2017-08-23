package layout

// NodeSet is a dense node set
type NodeSet []bool

// NewNodeSet returns new set for n nodes
func NewNodeSet(n int) NodeSet { return make(NodeSet, n) }

// Add includes node in set
func (nodes NodeSet) Add(n *Node) { nodes[n.ID] = true }

// Remove removes node from set
func (nodes NodeSet) Remove(n *Node) { nodes[n.ID] = false }

// Contains whether node is in set
func (nodes NodeSet) Contains(n *Node) bool { return nodes[n.ID] }

// Include includes node in set, returns false when node already exists
func (nodes NodeSet) Include(n *Node) bool {
	if nodes.Contains(n) {
		return false
	}
	nodes.Add(n)
	return true
}

// Nodes is a list of node identifiers
type Nodes []*Node

// Clear clears the list
func (nodes *Nodes) Clear() { *nodes = nil }

// Clone makes a clone of the list
func (nodes *Nodes) Clone() *Nodes {
	result := make(Nodes, len(*nodes))
	copy(result, *nodes)
	return &result
}

// Contains returns whether node is present in this list
func (nodes *Nodes) Contains(node *Node) bool { return nodes.IndexOf(node) >= 0 }

// Append adds node to the list without checking for duplicates
func (nodes *Nodes) Append(node *Node) { *nodes = append(*nodes, node) }

// Includes adds node to the list if it already doesn't contain it
func (nodes *Nodes) Include(node *Node) {
	if !nodes.Contains(node) {
		nodes.Append(node)
	}
}

// Remove removes node, including any duplicates
func (nodes *Nodes) Remove(node *Node) {
	i := nodes.IndexOf(node)
	for i >= 0 {
		nodes.Delete(i)
		i = nodes.indexOfAt(node, i)
	}
}

// Delete deletes
func (nodes *Nodes) Delete(i int) { *nodes = append((*nodes)[:i], (*nodes)[i+1:]...) }

// Normalize sorts and removes duplicates from the list
func (nodes *Nodes) Normalize() {
	nodes.SortBy(func(a, b *Node) bool {
		return a.ID < b.ID
	})
	// sort.Slice(*nodes, func(i, k int) bool {
	// 	return (*nodes)[i].ID < (*nodes)[k].ID
	// })

	{ // remove duplicates from sorted array
		var p *Node
		unique := (*nodes)[:0]
		for _, n := range *nodes {
			if p != n {
				unique = append(unique, n)
				p = n
			}
		}
		*nodes = unique
	}
}

// SortDescending sorts nodes in descending order of outdegree
func (nodes Nodes) SortDescending() Nodes {
	nodes.SortBy(func(a, b *Node) bool {
		if a.OutDegree() == b.OutDegree() {
			return a.InDegree() < b.InDegree()
		}
		return a.OutDegree() > b.OutDegree()
	})

	return nodes
}

// IndexOf finds the node index in this list
func (nodes *Nodes) IndexOf(node *Node) int {
	for i, x := range *nodes {
		if x == node {
			return i
		}
	}
	return -1
}

// indexOfAt finds the node index starting from offset
func (nodes *Nodes) indexOfAt(node *Node, offset int) int {
	for i, x := range (*nodes)[offset:] {
		if x == node {
			return offset + i
		}
	}
	return -1
}
