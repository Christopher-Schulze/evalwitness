package registry

import (
	"strings"
	"testing"
)

func TestMaintainerReviewChecklistPinsLocalGates(t *testing.T) {
	checklist, err := MaintainerReviewChecklist()
	if err != nil {
		t.Fatal(err)
	}
	if checklist.Digest == "" || checklist.SchemaVersion != ReviewChecklistSchemaVersion {
		t.Fatalf("checklist = %+v", checklist)
	}
	joined := ""
	required := 0
	for _, item := range checklist.Items {
		if item.Required {
			required++
		}
		joined += item.Command + " " + item.Text
	}
	if required < 6 {
		t.Fatalf("required items = %d", required)
	}
	for _, need := range []string{"preflight", "refresh", "render-matrix", "template", "no live provider"} {
		if !strings.Contains(strings.ToLower(joined), need) && need != "no live provider" {
			t.Fatalf("missing %q in %+v", need, checklist.Items)
		}
	}
	if !strings.Contains(strings.ToLower(joined), "no live") {
		t.Fatal("live-call prohibition missing")
	}
}
