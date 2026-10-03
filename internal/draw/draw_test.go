package draw

import "testing"

// TestApproxLineWidth checks that wide characters are estimated an em
// wide and marks on a character take no room.
func TestApproxLineWidth(t *testing.T) {
	const size = 10
	if got := ApproxLineWidth("漢字", size); got != 2*size {
		t.Errorf("two wide characters are %v wide, want %v", got, 2*size)
	}
	if plain, marked := ApproxLineWidth("e", size), ApproxLineWidth("é", size); plain != marked {
		t.Errorf("a combining mark widens %v to %v", plain, marked)
	}
}
