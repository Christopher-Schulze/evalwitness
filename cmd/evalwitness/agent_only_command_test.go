package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentStudyCommandRejectsInvalidInvocation(t *testing.T) {
	if code := runAgentStudy(nil); code != 2 {
		t.Fatalf("missing agent-study command returned %d", code)
	}
	if code := runAgentStudy([]string{"unknown"}); code != 2 {
		t.Fatalf("unknown agent-study command returned %d", code)
	}
	if code := runAgentStudyValidate(nil); code != 2 {
		t.Fatalf("missing agent-study validation input returned %d", code)
	}
}

func TestAgentStudyDestinationNeverOverwrites(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "study.json")
	if code := writeAgentStudyDestination("test", destination, []byte("first\n")); code != 0 {
		t.Fatalf("initial agent-study destination write returned %d", code)
	}
	if code := writeAgentStudyDestination("test", destination, []byte("second\n")); code == 0 {
		t.Fatal("agent-study destination overwrite was accepted")
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "first\n" {
		t.Fatalf("agent-study destination changed after rejected overwrite: %q", content)
	}
}
