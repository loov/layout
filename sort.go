package layout

import "sort"

type nodesorter struct {
	graph *Graph
	order NodeIDs
	less  func(a, b *Node) bool
}

func (ns *nodesorter) Len() int      { return len(ns.order) }
func (ns *nodesorter) Swap(i, k int) { ns.order[i], ns.order[k] = ns.order[k], ns.order[i] }
func (ns *nodesorter) Less(i, k int) bool {
	a, b := ns.graph.Nodes[ns.order[i]], ns.graph.Nodes[ns.order[k]]
	return ns.less(a, b)
}

func (graph *Graph) Sort(order NodeIDs, less func(a, b *Node) bool) {
	sort.Sort(&nodesorter{
		graph: graph,
		order: order,
		less:  less,
	})
}
