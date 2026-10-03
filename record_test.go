package layout

import "testing"

func TestParseRecord(t *testing.T) {
	rec := ParseRecord("<f0> left|<f1> mid\\nline|{top|bottom}")
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
	rec := ParseRecord(`C:\\new|x`)
	if len(rec.Fields) != 2 || rec.Fields[0].Text != `C:\new` {
		t.Errorf("got %+v, want C:\\new first", rec.Fields[0])
	}
}
