package hier

import (
	"math/rand"
	"slices"
	"testing"
)

// TestOrderOptimal measures how many more crossings OrderRanksN leaves
// than the fewest possible, with and without groups, on small random
// graphs through the pipeline, by a branch and bound over the orders of
// every rank; run with -v to see.
func TestOrderOptimal(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	for _, groups := range []bool{false, true} {
		rng := rand.New(rand.NewSource(1))
		graphs, optimal := 0, 0
		heuristic, exact := float32(0), float32(0)
		for graphs < 100 {
			graph := GenerateRandomGraph(12+rng.Intn(10), 0.15+0.1*rng.Float64(), rng)
			Decycle(graph)
			Rank(graph)
			AddVirtuals(graph)
			if !searchable(graph) {
				continue
			}
			OrderRanksN(graph, DefaultOrderIterations, groups)
			got := graph.TotalCrossings()
			best := fewestCrossings(graph, got)
			graphs++
			heuristic += got
			exact += best
			if best == got {
				optimal++
			}
		}
		t.Logf("groups %v: %d graphs, %d optimal, %v crossings against fewest %v", groups, graphs, optimal, heuristic, exact)
	}
}

// searchable reports whether every order of the ranks of graph is few
// enough to search
func searchable(graph *Graph) bool {
	orders := 1.0
	for _, layer := range graph.ByRank {
		if len(layer) > 9 {
			return false
		}
		for k := 2; k <= len(layer); k++ {
			orders *= float64(k)
		}
	}
	return orders <= 1e14
}

// fewestCrossings returns the fewest crossings of any order of the ranks of
// graph, or bound when none has fewer. It builds each rank left to right:
// the edges into a node placed cross those of the nodes before it a known
// amount, and every pair of nodes not yet placed at least the fewer of
// either way around.
func fewestCrossings(graph *Graph, bound float32) float32 {
	best := bound
	layers := graph.ByRank
	var search func(r int, sum float32)
	search = func(r int, sum float32) {
		if r == len(layers) {
			best = min(best, sum)
			return
		}
		nodes := slices.Clone(layers[r])
		n := len(nodes)
		// c[i][j] is what the edges into nodes i and j cross with i left of j
		c := make([][]float32, n)
		for i, u := range nodes {
			c[i] = make([]float32, n)
			for j, v := range nodes {
				for _, w := range u.In {
					for _, z := range v.In {
						if z.Pos < w.Pos {
							c[i][j] += graph.Weight(w, u) * graph.Weight(z, v)
						}
					}
				}
			}
		}

		order := make([]int, 0, n)
		var place func(used uint32, cost float32)
		place = func(used uint32, cost float32) {
			if len(order) == n {
				// a mirrored first rank crosses the same
				if r == 0 && n > 1 && order[0] > order[n-1] {
					return
				}
				for i, k := range order {
					layers[r][i] = nodes[k]
				}
				layers[r].assignPos()
				if r+1 < len(layers) && cost+graph.nextBound(layers[r+1]) >= best {
					return
				}
				search(r+1, cost)
				return
			}
			for v := range n {
				if used&(1<<v) != 0 {
					continue
				}
				next := cost
				for _, u := range order {
					next += c[u][v]
				}
				rest := used | 1<<v
				lower := next
				for q := range n {
					if rest&(1<<q) != 0 {
						continue
					}
					for _, p := range order {
						lower += c[p][q]
					}
					lower += c[v][q]
					for q2 := q + 1; q2 < n; q2++ {
						if rest&(1<<q2) == 0 {
							lower += min(c[q][q2], c[q2][q])
						}
					}
				}
				if lower >= best {
					continue
				}
				order = append(order, v)
				place(rest, next)
				order = order[:len(order)-1]
			}
		}
		place(0, sum)
		copy(layers[r], nodes)
		layers[r].assignPos()
	}
	search(0, 0)
	return best
}

// nextBound is the fewest crossings the edges into layer can have with
// the rank above it ordered: every pair of its nodes crosses at least the
// fewer of either way around
func (graph *Graph) nextBound(layer Nodes) float32 {
	total := float32(0)
	for i, u := range layer {
		for _, v := range layer[i+1:] {
			uv, vu := float32(0), float32(0)
			for _, w := range u.In {
				for _, z := range v.In {
					c := graph.Weight(w, u) * graph.Weight(z, v)
					if z.Pos < w.Pos {
						uv += c
					} else if z.Pos > w.Pos {
						vu += c
					}
				}
			}
			total += min(uv, vu)
		}
	}
	return total
}

// permute calls visit with nodes in every order, by Heap's algorithm, and
// leaves them in an order of them
func permute(nodes Nodes, visit func()) {
	var heap func(k int)
	heap = func(k int) {
		if k <= 1 {
			visit()
			return
		}
		for i := range k - 1 {
			heap(k - 1)
			if k%2 == 0 {
				nodes[i], nodes[k-1] = nodes[k-1], nodes[i]
			} else {
				nodes[0], nodes[k-1] = nodes[k-1], nodes[0]
			}
		}
		heap(k - 1)
	}
	heap(len(nodes))
}
