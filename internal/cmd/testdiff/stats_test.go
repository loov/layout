package main

import "testing"

func TestTextStats(t *testing.T) {
	got := textStats(`
          ╭───╮
          │ R │
          ╰┬┬┬╯
    ╭──────╯│╰──────╮
    │╭──────╂──────╮│
    ▼│      ▼      ▼▼
  ╭──┴╮   ╭───╮  ╭───╮
  │ A ├──▶│ B │  │ C │
  ╰───╯   ╰───╯  ╰───╯
 ┌ group ┈┈┈┐
 ┊ ┌──────┐ ┊
 ┊ │ x    │ ┊
 ┊ │──────│ ┊
 ┊ │ y    │ ┊
 ┊ └──────┘ ┊
 └┈┈┈┈┈┈┈┈┈┈┘
`)
	// bends: ╭╯ ╰╮ of R's outer edges, ╭╮ of the arc over B
	want := map[string]float64{"corners": 6, "crossings": 1, "length": 42}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}
