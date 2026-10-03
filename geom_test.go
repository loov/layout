package layout_test

import (
	"github.com/loov/layout"
	"testing"
)

func TestTwipsConvertToPoints(t *testing.T) {
	if got := layout.Length(1440 * layout.Twip); got != layout.Inch {
		t.Fatalf("1440 twips = %v points, want %v", got, layout.Inch)
	}
	if got := layout.Length(layout.Twip); got != 0.05 {
		t.Fatalf("1 twip = %v points, want 0.05", got)
	}
}
