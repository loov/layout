package draw

import "testing"

func TestParseRecord(t *testing.T) {
	rec := ParseRecord("<f0> left|<f1> mid\\nline|{top|bottom}", false)
	if len(rec.Fields) != 3 || rec.Vertical {
		t.Fatalf("expected 3 horizontal fields, got %+v", rec)
	}
	if rec.Fields[0].Text != "left" || rec.Fields[1].Text != "mid\nline" {
		t.Errorf("text fields wrong: %q %q", rec.Fields[0].Text, rec.Fields[1].Text)
	}
	group := rec.Fields[2]
	if !group.Vertical || len(group.Fields) != 2 || group.Fields[1].Text != "bottom" {
		t.Errorf("group wrong: %+v", group)
	}
}

// TestParseRecordEscapedBackslash checks that an escaped backslash stays
// a backslash rather than starting a line break with the next letter.
func TestParseRecordEscapedBackslash(t *testing.T) {
	rec := ParseRecord(`C:\\new|x`, false)
	if len(rec.Fields) != 2 || rec.Fields[0].Text != `C:\new` {
		t.Errorf("got %+v, want C:\\new first", rec.Fields[0])
	}
}

// TestTableRecordPorts checks that cells without a border between them
// share a field, unless the lower one names a port, which a field can
// only have at its start.
func TestTableRecordPorts(t *testing.T) {
	label := `<<table cellborder="1"><tr><td port="a" sides="ltr">one</td></tr><tr><td sides="lbr">two</td></tr><tr><td port="c" sides="ltr">three</td></tr><tr><td port="d" sides="lbr">four</td></tr></table>>`
	got, ok := TableRecord(label, true)
	if want := "<a>one\ntwo|<c>three|<d>four"; !ok || got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
