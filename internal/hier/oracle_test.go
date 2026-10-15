package hier

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestSimplexBruteForce compares network simplex with the cheapest ranks
// found by enumerating every rank up to the longest a tight solution can
// reach, on small random DAGs with weights, minimum lengths and zero
// weight edges, the vertices numbered in any order as positioning does.
func TestSimplexBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	const maxMinLen = 2
	for iter := range 500 {
		n := 2 + rng.IntN(6)
		perm := rng.Perm(n) // topological order to vertex
		s := &simplex{n: int32(n)}
		for range rng.IntN(3 * n) {
			a, b := rng.IntN(n), rng.IntN(n)
			if a == b {
				continue
			}
			a, b = min(a, b), max(a, b)
			s.addEdge(int32(perm[a]), int32(perm[b]), float32(rng.IntN(4)), int32(rng.IntN(maxMinLen+1)))
		}

		s.run()

		// edges by their head, which comes later in topological order
		into := make([][]int, n)
		for e := range s.head {
			into[s.head[e]] = append(into[s.head[e]], e)
		}
		cost := func(rank []int32) (float32, bool) {
			c := float32(0)
			for e := range s.tail {
				d := rank[s.head[e]] - rank[s.tail[e]]
				if d < s.minlen[e] {
					return 0, false
				}
				c += s.weight[e] * float32(d)
			}
			return c, true
		}
		got, ok := cost(s.rank)
		if !ok {
			t.Fatalf("iter %d: infeasible ranks %v", iter, s.rank)
		}

		// look only for ranks cheaper than the solver's; with no negative
		// weights a partial sum that reaches it can stop
		best := got
		rank := make([]int32, n)
		maxRank := int32(maxMinLen * (n - 1))
		// in topological order every vertex starts where its edges in
		// allow it
		var enumerate func(a int, sum float32)
		enumerate = func(a int, sum float32) {
			if sum >= best {
				return
			}
			if a == n {
				best = sum
				return
			}
			v := perm[a]
			lo := int32(0)
			for _, e := range into[v] {
				lo = max(lo, rank[s.tail[e]]+s.minlen[e])
			}
			for r := lo; r <= maxRank; r++ {
				rank[v] = r
				c := sum
				for _, e := range into[v] {
					c += s.weight[e] * float32(r-rank[s.tail[e]])
				}
				enumerate(a+1, c)
			}
		}
		enumerate(0, 0)
		if got != best {
			t.Fatalf("iter %d: cost %v, cheapest %v", iter, got, best)
		}
		lo := int32(math.MaxInt32)
		for _, r := range s.rank {
			lo = min(lo, r)
		}
		if lo != 0 {
			t.Fatalf("iter %d: lowest rank %d", iter, lo)
		}
	}
}

// randomLayers returns a proper layered graph: edges only between
// adjacent ranks, some doubled, flat edges within ranks, and when asked,
// weights and ports; ports stay inside ±0.5 so that the ends on
// neighboring nodes keep their order.
func randomLayers(rng *rand.Rand, weights, ports bool) *Graph {
	graph := NewGraph()
	graph.ByRank = make([]Nodes, 2+rng.IntN(3))
	for r := range graph.ByRank {
		for range 1 + rng.IntN(5) {
			node := graph.AddNode()
			node.Rank = r
			graph.ByRank[r] = append(graph.ByRank[r], node)
		}
	}

	offsets := []float32{-0.4, -0.2, 0, 0.2, 0.4}
	for r := 1; r < len(graph.ByRank); r++ {
		for _, u := range graph.ByRank[r-1] {
			for _, v := range graph.ByRank[r] {
				if rng.IntN(5) >= 2 {
					continue
				}
				graph.AddEdge(u, v)
				if rng.IntN(8) == 0 {
					graph.AddEdge(u, v)
				}
				if weights {
					graph.SetWeight(u, v, float32(1+rng.IntN(3)))
				}
				if ports {
					if graph.Ports == nil {
						graph.Ports = map[[2]ID][2]float32{}
					}
					graph.Ports[[2]ID{u.ID, v.ID}] = [2]float32{offsets[rng.IntN(5)], offsets[rng.IntN(5)]}
				}
			}
		}
	}

	for _, layer := range graph.ByRank {
		if len(layer) > 2 && rng.IntN(2) == 0 {
			a, b := rng.IntN(len(layer)), rng.IntN(len(layer))
			if a != b {
				graph.Flat = append(graph.Flat, [2]*Node{layer[a], layer[b]})
			}
		}
	}

	graph.assignPos()
	return graph
}

// segment is an edge between adjacent ranks as drawn: where it leaves
// the upper rank, where it reaches the lower one, and its weight
type segment struct{ up, lo, weight float32 }

// pairCrossings counts the weighted crossings of every segment of a with
// every segment of b
func pairCrossings(a, b []segment) float32 {
	total := float32(0)
	for _, x := range a {
		for _, y := range b {
			if (x.up-y.up)*(x.lo-y.lo) < 0 {
				total += x.weight * y.weight
			}
		}
	}

	return total
}

// TestCrossingsBruteForce checks the crossing counters against counting
// every pair of edges, with and without weights and ports.
func TestCrossingsBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for iter := range 2000 {
		weights := iter%2 == 1
		ports := iter%4 >= 2
		graph := randomLayers(rng, weights, ports)
		port := func(src, dst *Node) [2]float32 { return graph.Ports[[2]ID{src.ID, dst.ID}] }

		total := float32(0)
		for r := 1; r < len(graph.ByRank); r++ {
			var segs []segment
			for _, u := range graph.ByRank[r-1] {
				for _, v := range u.Out {
					p := port(u, v)
					segs = append(segs, segment{float32(u.Pos) + p[0], float32(v.Pos) + p[1], graph.Weight(u, v)})
				}
			}
			want := float32(0)
			for i := range segs {
				want += pairCrossings(segs[i:i+1], segs[i+1:])
			}
			total += want
			if got := graph.bilayerCrossings(graph.ByRank[r-1], graph.ByRank[r]); got != want {
				t.Fatalf("iter %d rank %d: bilayerCrossings %v, want %v", iter, r, got, want)
			}
		}

		for _, edge := range graph.Flat {
			lo, hi := min(edge[0].Pos, edge[1].Pos), max(edge[0].Pos, edge[1].Pos)
			for _, node := range graph.ByRank[edge[0].Rank] {
				if lo < node.Pos && node.Pos < hi {
					total++
				}
			}
		}

		if got := graph.TotalCrossings(); got != total {
			t.Fatalf("iter %d: TotalCrossings %v, want %v", iter, got, total)
		}

		for _, layer := range graph.ByRank {
			for _, node := range layer {
				node.inPos, node.outPos = node.inPos[:0], node.outPos[:0]
				for _, src := range node.In {
					node.inPos = append(node.inPos, int32(src.Pos))
				}
				for _, dst := range node.Out {
					node.outPos = append(node.outPos, int32(dst.Pos))
				}
			}
		}

		// the edges of node to the rank above and below, with node at x
		ends := func(node *Node, x float32, plain bool) (in, out []segment) {
			for _, src := range node.In {
				p := port(src, node)
				w := graph.Weight(src, node)
				if plain {
					p = [2]float32{}
					w = 1
				}
				in = append(in, segment{float32(src.Pos) + p[0], x + p[1], w})
			}
			for _, dst := range node.Out {
				p := port(node, dst)
				w := graph.Weight(node, dst)
				if plain {
					p = [2]float32{}
					w = 1
				}
				out = append(out, segment{x + p[0], float32(dst.Pos) + p[1], w})
			}
			return in, out
		}

		// crossings of the edges of u with those of v, u left of v
		between := func(u, v *Node, plain bool) float32 {
			uin, uout := ends(u, 0, plain)
			vin, vout := ends(v, 1, plain)
			return pairCrossings(uin, vin) + pairCrossings(uout, vout)
		}
		for _, layer := range graph.ByRank {
			for i, u := range layer {
				for _, v := range layer[i+1:] {
					wantUV, wantVU := between(u, v, false), between(v, u, false)
					if uv, vu := graph.crossingsBothWays(u, v); uv != wantUV || vu != wantVU {
						t.Fatalf("iter %d: crossingsBothWays(%v, %v) = %v, %v, want %v, %v", iter, u, v, uv, vu, wantUV, wantVU)
					}
					wantUV, wantVU = between(u, v, true), between(v, u, true)
					if uv, vu := crossingsByPos(u, v); uv != wantUV || vu != wantVU {
						t.Fatalf("iter %d: crossingsByPos(%v, %v) = %v, %v, want %v, %v", iter, u, v, uv, vu, wantUV, wantVU)
					}
				}
			}
		}
	}
}
