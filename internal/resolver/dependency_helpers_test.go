package resolver

import "testing"

func TestSplitConstraintStringPreservesOrder(t *testing.T) {
	input := ">= 1.15.7, != 1.16.7, != 1.16.6, != 1.16.0"
	want := []string{">= 1.15.7", "!= 1.16.7", "!= 1.16.6", "!= 1.16.0"}

	got := splitConstraintString(input)
	if len(got) != len(want) {
		t.Fatalf("splitConstraintString length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitConstraintString[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitConstraintStringHandlesAmpersand(t *testing.T) {
	input := ">= 1.0 & < 2.0"
	want := []string{">= 1.0", "< 2.0"}

	got := splitConstraintString(input)
	if len(got) != len(want) {
		t.Fatalf("splitConstraintString length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitConstraintString[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
