package text

import (
	"testing"
	"unicode"
)

// TestArms checks that the table arms reads from has every character with
// arms
func TestArms(t *testing.T) {
	for r := range rune(unicode.MaxRune + 1) {
		if got, want := arms(r), lookupArms(r); got != want {
			t.Errorf("arms(%q) = %b, want %b", r, got, want)
		}
	}
}
