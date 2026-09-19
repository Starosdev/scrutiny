package utils

import "testing"

func TestLeftPad2Len(t *testing.T) {
	if got, want := LeftPad2Len("v1", " ", 5), "   v1"; got != want {
		t.Fatalf("LeftPad2Len = %q, want %q", got, want)
	}
}

func TestStripIndent(t *testing.T) {
	if got, want := StripIndent("\tfirst\n\tsecond"), "first\nsecond"; got != want {
		t.Fatalf("StripIndent = %q, want %q", got, want)
	}
}
