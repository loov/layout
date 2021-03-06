package hier

// Sort implementation is a modification of http://golang.org/pkg/sort/#Sort
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at http://golang.org/LICENSE.

// SortBy sorts nodes in place
func (nodes *Nodes) SortBy(less func(*Node, *Node) bool) {
	// Switch to heapsort if depth of 2*ceil(lg(n+1)) is reached.
	n := len(*nodes)
	maxDepth := 0
	for i := n; i > 0; i >>= 1 {
		maxDepth++
	}
	maxDepth *= 2
	nodes._quickSortNodeSlice(less, 0, n, maxDepth)
}

// Sort implementation based on http://golang.org/pkg/sort/#Sort

func (rcv Nodes) _swapNodeSlice(a, b int) {
	rcv[a], rcv[b] = rcv[b], rcv[a]
}

// Insertion sort
func (rcv Nodes) _insertionSortNodeSlice(less func(*Node, *Node) bool, a, b int) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && less(rcv[j], rcv[j-1]); j-- {
			rcv._swapNodeSlice(j, j-1)
		}
	}
}

// siftDown implements the heap property on rcv[lo, hi).
// first is an offset into the array where the root of the heap lies.
func (rcv Nodes) _siftDownNodeSlice(less func(*Node, *Node) bool, lo, hi, first int) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			break
		}
		if child+1 < hi && less(rcv[first+child], rcv[first+child+1]) {
			child++
		}
		if !less(rcv[first+root], rcv[first+child]) {
			return
		}
		rcv._swapNodeSlice(first+root, first+child)
		root = child
	}
}

func (rcv Nodes) _heapSortNodeSlice(less func(*Node, *Node) bool, a, b int) {
	first := a
	lo := 0
	hi := b - a

	// Build heap with greatest element at top.
	for i := (hi - 1) / 2; i >= 0; i-- {
		rcv._siftDownNodeSlice(less, i, hi, first)
	}

	// Pop elements, largest first, into end of rcv._
	for i := hi - 1; i >= 0; i-- {
		rcv._swapNodeSlice(first, first+i)
		rcv._siftDownNodeSlice(less, lo, i, first)
	}
}

// Quicksort, following Bentley and McIlroy,
// Engineering a Sort Function, SP&E November 1993.

// medianOfThree moves the median of the three values rcv[a], rcv[b], rcv[c] into rcv[a].
func (rcv Nodes) _medianOfThreeNodeSlice(less func(*Node, *Node) bool, a, b, c int) {
	m0 := b
	m1 := a
	m2 := c
	// bubble sort on 3 elements
	if less(rcv[m1], rcv[m0]) {
		rcv._swapNodeSlice(m1, m0)
	}
	if less(rcv[m2], rcv[m1]) {
		rcv._swapNodeSlice(m2, m1)
	}
	if less(rcv[m1], rcv[m0]) {
		rcv._swapNodeSlice(m1, m0)
	}
	// now rcv[m0] <= rcv[m1] <= rcv[m2]
}

func (rcv Nodes) _swapRangeNodeSlice(a, b, n int) {
	for i := 0; i < n; i++ {
		rcv._swapNodeSlice(a+i, b+i)
	}
}

func (rcv Nodes) _doPivotNodeSlice(less func(*Node, *Node) bool, lo, hi int) (midlo, midhi int) {
	m := lo + (hi-lo)/2 // Written like this to avoid integer overflow.
	if hi-lo > 40 {
		// Tukey's Ninther, median of three medians of three.
		s := (hi - lo) / 8
		rcv._medianOfThreeNodeSlice(less, lo, lo+s, lo+2*s)
		rcv._medianOfThreeNodeSlice(less, m, m-s, m+s)
		rcv._medianOfThreeNodeSlice(less, hi-1, hi-1-s, hi-1-2*s)
	}
	rcv._medianOfThreeNodeSlice(less, lo, m, hi-1)

	// Invariants are:
	//	rcv[lo] = pivot (set up by ChoosePivot)
	//	rcv[lo <= i < a] = pivot
	//	rcv[a <= i < b] < pivot
	//	rcv[b <= i < c] is unexamined
	//	rcv[c <= i < d] > pivot
	//	rcv[d <= i < hi] = pivot
	//
	// Once b meets c, can swap the "= pivot" sections
	// into the middle of the slice.
	pivot := lo
	a, b, c, d := lo+1, lo+1, hi, hi
	for {
		for b < c {
			if less(rcv[b], rcv[pivot]) { // rcv[b] < pivot
				b++
			} else if !less(rcv[pivot], rcv[b]) { // rcv[b] = pivot
				rcv._swapNodeSlice(a, b)
				a++
				b++
			} else {
				break
			}
		}
		for b < c {
			if less(rcv[pivot], rcv[c-1]) { // rcv[c-1] > pivot
				c--
			} else if !less(rcv[c-1], rcv[pivot]) { // rcv[c-1] = pivot
				rcv._swapNodeSlice(c-1, d-1)
				c--
				d--
			} else {
				break
			}
		}
		if b >= c {
			break
		}
		// rcv[b] > pivot; rcv[c-1] < pivot
		rcv._swapNodeSlice(b, c-1)
		b++
		c--
	}

	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	n := min(b-a, a-lo)
	rcv._swapRangeNodeSlice(lo, b-n, n)

	n = min(hi-d, d-c)
	rcv._swapRangeNodeSlice(c, hi-n, n)

	return lo + b - a, hi - (d - c)
}

func (rcv Nodes) _quickSortNodeSlice(less func(*Node, *Node) bool, a, b, maxDepth int) {
	for b-a > 7 {
		if maxDepth == 0 {
			rcv._heapSortNodeSlice(less, a, b)
			return
		}
		maxDepth--
		mlo, mhi := rcv._doPivotNodeSlice(less, a, b)
		// Avoiding recursion on the larger subproblem guarantees
		// a stack depth of at most lg(b-a).
		if mlo-a < b-mhi {
			rcv._quickSortNodeSlice(less, a, mlo, maxDepth)
			a = mhi // i.e., rcv._quickSortNodeSlice(mhi, b)
		} else {
			rcv._quickSortNodeSlice(less, mhi, b, maxDepth)
			b = mlo // i.e., rcv._quickSortNodeSlice(a, mlo)
		}
	}
	if b-a > 1 {
		rcv._insertionSortNodeSlice(less, a, b)
	}
}
