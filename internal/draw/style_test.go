package draw

import (
	"reflect"
	"testing"
)

// TestStyledLabel checks that the markup of an HTML-like label that styles
// its text marks the spans of its lines, and takes no room
func TestStyledLabel(t *testing.T) {
	label := PlainLabel(`<<b>HEAD </b>of <font color="red">the<br/><i>current</i></font> branch>`)
	lines := Lines(label)
	if len(lines) != 2 {
		t.Fatalf("lines %q", label)
	}
	want := [][]Span{
		{{"HEAD ", Style{Bold: true}}, {"of ", Style{}}, {"the", Style{Color: "red"}}},
		{{"current", Style{Italic: true, Color: "red"}}, {" branch", Style{}}},
	}
	for i, line := range lines {
		if got := Spans(line.Text); !reflect.DeepEqual(got, want[i]) {
			t.Errorf("line %d: spans %+v, want %+v", i, got, want[i])
		}
	}
	if got := Columns(lines[0].Text); got != len("HEAD of the") {
		t.Errorf("columns %d", got)
	}
}
