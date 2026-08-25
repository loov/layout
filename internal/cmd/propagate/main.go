// Command propagate is an experiment in computing reachability layers of
// a small graph with bitset edge tables.
package main

import (
	"fmt"
	"strings"
)

// EdgeCount is the number of nodes; EdgeMask has one bit per node.
const (
	EdgeCount = 6
	EdgeMask  = (1 << EdgeCount) - 1
)

// EdgeTable holds, for every pair of nodes, a bitset of the nodes
// through which they are connected.
type EdgeTable [EdgeCount][EdgeCount]byte

// Edge marks a direct connection from x to y.
func (Z *EdgeTable) Edge(x, y int) {
	(*Z)[x][y] = EdgeMask
}

// DeleteOutbound clears each node's own bit from its outgoing connections.
func (Z *EdgeTable) DeleteOutbound() {
	for x := range EdgeCount {
		for y := range EdgeCount {
			(*Z)[x][y] &^= (1 << byte(x))
		}
	}
}

// DeleteInbound clears each node's own bit from its incoming connections.
func (Z *EdgeTable) DeleteInbound() {
	for x := range EdgeCount {
		for y := range EdgeCount {
			(*Z)[x][y] &^= (1 << byte(y))
		}
	}
}

// Or sets Z to the element-wise union of A and B.
func (Z *EdgeTable) Or(A, B *EdgeTable) {
	for x := range EdgeCount {
		for y := range EdgeCount {
			(*Z)[x][y] = (*A)[x][y] | (*B)[x][y]
		}
	}
}

// And sets Z to the element-wise intersection of A and B.
func (Z *EdgeTable) And(A, B *EdgeTable) {
	for x := range EdgeCount {
		for y := range EdgeCount {
			(*Z)[x][y] = (*A)[x][y] & (*B)[x][y]
		}
	}
}

// Mul sets Z to the boolean matrix product of A and B, recording the
// intermediate nodes in the bitsets.
func (Z *EdgeTable) Mul(A, B *EdgeTable) {
	for x := range EdgeCount {
		for y := range EdgeCount {
			t := byte(0)
			for k := range EdgeCount {
				t |= (*A)[x][k] & (*B)[k][y]
			}
			(*Z)[x][y] = t
		}
	}
}

// Print writes the table as hex.
func (Z *EdgeTable) Print() {
	for x := range EdgeCount {
		for y := range EdgeCount {
			v := (*Z)[x][y]
			if v == 0 {
				fmt.Printf(" · ")
			} else {
				fmt.Printf("%2d ", v)
			}
		}
		fmt.Printf("\n")
	}
}

// PrintBit writes the table as binary.
func (Z *EdgeTable) PrintBit() {
	for x := range EdgeCount {
		for y := range EdgeCount {
			v := (*Z)[x][y]
			fmt.Printf("%2d ", v)
			s := fmt.Sprintf("%08b ", v)
			s = strings.Replace(s, "0", "░", -1)
			s = strings.Replace(s, "1", "█", -1)
			fmt.Print(s)
		}
		fmt.Printf("\n")
	}
}

// PrintBool writes the table as a connectivity matrix.
func (Z *EdgeTable) PrintBool() {
	fmt.Printf("  ")
	for y := range EdgeCount {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("\n")
	for x := range EdgeCount {
		fmt.Printf("%d ", x)
		for y := range EdgeCount {
			v := (*Z)[x][y]
			if v == 0 {
				fmt.Printf("░░")
			} else {
				fmt.Printf("██")
			}
		}
		fmt.Printf("\n")
	}
}

// PrintLayer writes the connectivity matrix restricted to bit n.
func (Z *EdgeTable) PrintLayer(n byte) {
	fmt.Printf("  ")
	for y := range EdgeCount {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("\n")
	for x := range EdgeCount {
		fmt.Printf("%d ", x)
		for y := range EdgeCount {
			v := ((*Z)[x][y] >> n) & 1
			if v == 0 {
				fmt.Printf("░░")
			} else {
				fmt.Printf("██")
			}
		}
		fmt.Printf("\n")
	}
}

// PrintSideBySideLayer writes bit n of three tables next to each other.
func PrintSideBySideLayer(A, B, C *EdgeTable, n byte) {
	fmt.Printf("  ")
	for y := range EdgeCount {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("   ")
	for y := range EdgeCount {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("   ")
	for y := range EdgeCount {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("\n")

	for x := range EdgeCount {
		fmt.Printf("%d ", x)

		for y := range EdgeCount {
			v := ((*A)[x][y] >> n) & 1
			if v == 0 {
				fmt.Printf("░░")
			} else {
				fmt.Printf("██")
			}
		}

		fmt.Printf(" %d ", x)

		for y := range EdgeCount {
			v := ((*B)[x][y] >> n) & 1
			if v == 0 {
				fmt.Printf("░░")
			} else {
				fmt.Printf("██")
			}
		}

		fmt.Printf(" %d ", x)

		for y := range EdgeCount {
			v := ((*C)[x][y] >> n) & 1
			if v == 0 {
				fmt.Printf("░░")
			} else {
				fmt.Printf("██")
			}
		}

		fmt.Printf("\n")
	}
}

// CountLayer counts entries that have bit n set.
func (Z *EdgeTable) CountLayer(n byte) int {
	total := 0
	for x := range EdgeCount {
		for y := range EdgeCount {
			total += int(((*Z)[x][y] >> n) & 1)
		}
	}
	return total
}

const (
	NodeA = iota
	NodeB
	NodeC
	NodeD
	NodeE
	NodeF
)

// Process repeatedly multiplies the table with itself, propagating
// connectivity until it stops changing.
func Process(input *EdgeTable) EdgeTable {
	var result, temp EdgeTable

	result = *input
	for range EdgeCount {
		temp.Mul(&result, input)
		result.Or(&result, &temp)
	}

	return result
}

func main() {
	var Input EdgeTable
	Input.Edge(NodeA, NodeB)
	Input.Edge(NodeA, NodeC)
	Input.Edge(NodeB, NodeD)
	Input.Edge(NodeC, NodeD)
	Input.Edge(NodeC, NodeE)
	Input.Edge(NodeD, NodeE)
	Input.Edge(NodeD, NodeF)
	Input.Edge(NodeE, NodeF)
	Input.Edge(NodeF, NodeA)

	Input.PrintBool()

	inbound := Input
	outbound := Input

	inbound.DeleteInbound()

	for layer := range EdgeCount {
		fmt.Println("------------------")
		inbound.PrintLayer(byte(layer))
	}

	inbound = Process(&inbound)

	outbound.DeleteOutbound()
	outbound = Process(&outbound)

	anded := inbound
	anded.And(&inbound, &outbound)

	fmt.Println("~~~~~~~~~~~~~~~~~~~~~")
	inbound.PrintBit()
	fmt.Println("~~~~~~~~~~~~~~~~~~~~~")
	outbound.PrintBit()
	fmt.Println("~~~~~~~~~~~~~~~~~~~~~")
	anded.PrintBit()

	for layer := range EdgeCount {
		fmt.Println("------------------")
		PrintSideBySideLayer(&inbound, &outbound, &anded, byte(layer))

		fmt.Println(
			"+ ",
			inbound.CountLayer(byte(layer)),
			outbound.CountLayer(byte(layer)),
			anded.CountLayer(byte(layer)),
		)
	}

}
