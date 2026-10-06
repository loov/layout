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

func TestDiamondSteps(t *testing.T) {
	for _, test := range []struct {
		label string
		want  int
	}{
		{"", 1},
		{"a", 2},
		{"lbl", 2},
		{"label", 3},
		{"a\nb", 3},
		{"one\ntwo\nthree", 4},
	} {
		if got := DiamondSteps(test.label); got != test.want {
			t.Errorf("DiamondSteps(%q) = %d, want %d", test.label, got, test.want)
		}
	}
}

func TestCornerDiamondSteps(t *testing.T) {
	for _, test := range []struct {
		label string
		want  int
	}{
		{"", 1},
		{"a", 2},
		{"lbl", 3},
		{"a\nb", 2},
	} {
		if got := CornerDiamondSteps(test.label); got != test.want {
			t.Errorf("CornerDiamondSteps(%q) = %d, want %d", test.label, got, test.want)
		}
	}
}

func TestDiamondSize(t *testing.T) {
	if cols, rows := DiamondSize(""); cols != 4 || rows != 4 {
		t.Errorf("DiamondSize(\"\") = %d, %d, want 4, 4", cols, rows)
	}
	if cols, rows := DiamondSize("lbl"); cols != 12 || rows != 6 {
		t.Errorf("DiamondSize(\"lbl\") = %d, %d, want 12, 6", cols, rows)
	}
}

func TestStepDiamondSteps(t *testing.T) {
	for _, test := range []struct {
		label string
		want  int
	}{
		{"", 1},
		{"ab", 1},
		{"lbl", 2},
		{"one\ntwo\nthree", 3},
	} {
		if got := StepDiamondSteps(test.label); got != test.want {
			t.Errorf("StepDiamondSteps(%q) = %d, want %d", test.label, got, test.want)
		}
	}
}
