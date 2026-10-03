package layout

import "testing"

// TestApproxTextWidth checks that wide characters are estimated an em
// wide and marks on a character take no room.
func TestApproxTextWidth(t *testing.T) {
	const size = 10
	if got := approxTextWidth("漢字", "", size).X; got != size {
		t.Errorf("two wide characters are %v wide, want %v", 2*got, 2*size)
	}
	if plain, marked := approxTextWidth("e", "", size), approxTextWidth("é", "", size); plain != marked {
		t.Errorf("a combining mark widens %v to %v", plain, marked)
	}
}
