package relation

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildPilotLineChangeIncludesEveryNonCommonLine(t *testing.T) {
	t.Parallel()

	change := buildPilotLineChange("a\nb\nreference\ncall\nz\n", "a\nb\ncall\nreference\nz\n", 1)
	if change.CommonPrefixLines != 2 || change.CommonSuffixLines != 1 {
		t.Fatalf("line change boundaries = %#v", change)
	}
	if !slices.Equal(change.OriginalChanged, []string{"reference", "call"}) || !slices.Equal(change.TransformedChanged, []string{"call", "reference"}) {
		t.Fatalf("line change omitted a non-common line: %#v", change)
	}
	if rendered := renderPilotLineChange(change); !strings.Contains(rendered, "- reference\n- call\n+ call\n+ reference") {
		t.Fatalf("rendered change = %q", rendered)
	}
}

func TestPilotLineChangeDetectsReferenceReviewSurface(t *testing.T) {
	t.Parallel()

	change := buildPilotLineChange("before\n[Event reference: result]\ncall\nafter", "before\ncall\n[Event reference: result]\nafter", 3)
	if !pilotLineChangeContains(change, "[Event reference:") {
		t.Fatal("reference wrapper change was not detected")
	}
}
