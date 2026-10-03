package layout

import (
	"slices"
	"testing"
)

func TestSeparate(t *testing.T) {
	for _, tc := range []struct {
		want   []Length
		weight []float64
		out    []Length
	}{
		{[]Length{0, 10, 20}, []float64{1, 1, 1}, []Length{0, 10, 20}},
		{[]Length{0, 0}, []float64{1, 1}, []Length{-5, 5}},
		{[]Length{0, 0}, []float64{1, 3}, []Length{-7.5, 2.5}},
		{[]Length{-5, 0, 5}, []float64{1, 1, 1}, []Length{-10, 0, 10}},
		{[]Length{90, 100}, []float64{1, 1}, []Length{85, 95}}, // clamped to hi
	} {
		got := separate(tc.want, tc.weight, 10, -100, 95)
		if !slices.Equal(got, tc.out) {
			t.Errorf("separate(%v, %v) = %v, want %v", tc.want, tc.weight, got, tc.out)
		}
	}
}
