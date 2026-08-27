package main

import (
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/replay"
)

func TestJointAbsoluteLocksIdenticalResponseAnalyzer(t *testing.T) {
	command, version := evalAnalysisIdentity("joint_absolute", "eval-terminal")
	want := []string{"evalwitness", "replay", "study", "analyze-identical-response"}
	if !slices.Equal(command, want) || version != replay.IdenticalResponseAnalysisSchemaVersion {
		t.Fatalf("analysis identity = %v %q, want %v %q", command, version, want, replay.IdenticalResponseAnalysisSchemaVersion)
	}
	command, version = evalAnalysisIdentity("absolute", "eval-terminal")
	if !slices.Equal(command, []string{"evalwitness", "eval-terminal"}) || version != evalArtifactSchemaVersion {
		t.Fatalf("default analysis identity drifted: %v %q", command, version)
	}
}
