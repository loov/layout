package main

import (
	"fmt"
	"strings"
)

const (
	EdgeCount = 6
	EdgeMask  = (1 << EdgeCount) - 1
)

type EdgeTable [EdgeCount][EdgeCount]byte

func (Z *EdgeTable) Edge(x, y int) {
	(*Z)[x][y] = EdgeMask
}

func (Z *EdgeTable) DeleteOutbound() {
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
			(*Z)[x][y] &^= (1 << byte(x))
		}
	}
}

func (Z *EdgeTable) DeleteInbound() {
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
			(*Z)[x][y] &^= (1 << byte(y))
		}
	}
}

func (Z *EdgeTable) Or(A, B *EdgeTable) {
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
			(*Z)[x][y] = (*A)[x][y] | (*B)[x][y]
		}
	}
}

func (Z *EdgeTable) And(A, B *EdgeTable) {
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
			(*Z)[x][y] = (*A)[x][y] & (*B)[x][y]
		}
	}
}

func (Z *EdgeTable) Mul(A, B *EdgeTable) {
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
			t := byte(0)
			for k := 0; k < EdgeCount; k++ {
				t |= (*A)[x][k] & (*B)[k][y]
			}
			(*Z)[x][y] = t
		}
	}
}

func (Z *EdgeTable) Print() {
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
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

func (Z *EdgeTable) PrintBit() {
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
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

func (Z *EdgeTable) PrintBool() {
	fmt.Printf("  ")
	for y := 0; y < EdgeCount; y++ {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("\n")
	for x := 0; x < EdgeCount; x++ {
		fmt.Printf("%d ", x)
		for y := 0; y < EdgeCount; y++ {
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

func (Z *EdgeTable) PrintLayer(n byte) {
	fmt.Printf("  ")
	for y := 0; y < EdgeCount; y++ {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("\n")
	for x := 0; x < EdgeCount; x++ {
		fmt.Printf("%d ", x)
		for y := 0; y < EdgeCount; y++ {
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

func PrintSideBySideLayer(A, B, C *EdgeTable, n byte) {
	fmt.Printf("  ")
	for y := 0; y < EdgeCount; y++ {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("   ")
	for y := 0; y < EdgeCount; y++ {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("   ")
	for y := 0; y < EdgeCount; y++ {
		fmt.Printf("%d ", y)
	}
	fmt.Printf("\n")

	for x := 0; x < EdgeCount; x++ {
		fmt.Printf("%d ", x)

		for y := 0; y < EdgeCount; y++ {
			v := ((*A)[x][y] >> n) & 1
			if v == 0 {
				fmt.Printf("░░")
			} else {
				fmt.Printf("██")
			}
		}

		fmt.Printf(" %d ", x)

		for y := 0; y < EdgeCount; y++ {
			v := ((*B)[x][y] >> n) & 1
			if v == 0 {
				fmt.Printf("░░")
			} else {
				fmt.Printf("██")
			}
		}

		fmt.Printf(" %d ", x)

		for y := 0; y < EdgeCount; y++ {
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

func (Z *EdgeTable) CountLayer(n byte) int {
	total := 0
	for x := 0; x < EdgeCount; x++ {
		for y := 0; y < EdgeCount; y++ {
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

func Process(input *EdgeTable) EdgeTable {
	var result, temp EdgeTable

	result = *input
	for i := 0; i < EdgeCount; i++ {
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

	for layer := 0; layer < EdgeCount; layer++ {
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

	for layer := 0; layer < EdgeCount; layer++ {
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
