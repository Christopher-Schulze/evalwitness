package drift

import "testing"

func TestDescriptiveThresholdsNotLocked(t *testing.T) {
	ths := DescriptiveThresholds()
	if len(ths) == 0 {
		t.Fatal("empty")
	}
	for _, th := range ths {
		if th.Locked {
			t.Fatalf("threshold %q should be descriptive not locked", th.Metric)
		}
		if th.Metric == "" {
			t.Fatal("metric empty")
		}
	}
}
