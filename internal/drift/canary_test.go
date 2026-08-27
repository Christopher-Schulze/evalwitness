package drift

import "testing"

func TestBuildPackDeterministic(t *testing.T) {
	canaries := []Canary{
		{ID: "b", Purpose: "topk", Task: "t2", RequestContract: "c", MaxCalls: 1, MaxTokens: 100, MaxTimeSeconds: 10, License: "MIT"},
		{ID: "a", Purpose: "align", Task: "t1", RequestContract: "c", MaxCalls: 1, MaxTokens: 100, MaxTimeSeconds: 10, License: "MIT"},
	}
	p1, err := BuildPack("v1", canaries)
	if err != nil {
		t.Fatalf("build %v", err)
	}
	p2, _ := BuildPack("v1", []Canary{canaries[1], canaries[0]})
	if p1.Digest != p2.Digest {
		t.Fatalf("not deterministic %s vs %s", p1.Digest, p2.Digest)
	}
	if p1.Canaries[0].ID != "a" {
		t.Fatalf("not sorted %v", p1.Canaries)
	}
	// Missing field must fail
	bad := []Canary{{ID: "", Purpose: "x", Task: "t", RequestContract: "c", MaxCalls: 1, MaxTokens: 1, MaxTimeSeconds: 1, License: "MIT"}}
	if _, err := BuildPack("v1", bad); err == nil {
		t.Fatal("expected error on empty id")
	}
}
