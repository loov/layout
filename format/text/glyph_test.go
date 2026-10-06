package text

import (
	"testing"
	"unicode"
)

// TestClass checks that the table class reads from has every character
// with arms or of a class
func TestClass(t *testing.T) {
	for r := range rune(unicode.MaxRune + 1) {
		if got, want := class(r), lookupClass(r); got != want {
			t.Errorf("class(%q) = %b, want %b", r, got, want)
		}
	}
}
