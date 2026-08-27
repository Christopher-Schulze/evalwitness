package mode

import "testing"

func TestSelectionMarginTreatsDecimalBoundaryAsTie(t *testing.T) {
	margin := 0.90 - 0.88
	if selectionMarginExceedsEpsilon(margin, 0.02) {
		t.Fatalf("binary floating-point margin %.18f crossed the exact epsilon boundary", margin)
	}
	if !selectionMarginExceedsEpsilon(0.021, 0.02) {
		t.Fatal("margin above epsilon did not select")
	}
}
