//go:build darwin || linux

package mutation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrustedValidatorTimeoutTerminatesDescendantProcessGroup(t *testing.T) {
	root := t.TempDir()
	validator := TrustedValidator{
		Spec:            ValidatorSpec{TimeoutMillis: 50, MaximumOutputBytes: 4_096},
		Executable:      "/bin/sh",
		Arguments:       []string{"-c", "(sleep 0.3; touch escaped.marker) & wait"},
		PassingExitCode: 0,
	}
	_, err := runTrustedValidator(context.Background(), validator, TaskEnvironment{Root: root})
	if err == nil {
		t.Fatal("trusted validator timeout was not reported")
	}
	time.Sleep(500 * time.Millisecond)
	_, statErr := os.Stat(filepath.Join(root, "escaped.marker"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("trusted validator descendant escaped process-group termination: %v", statErr)
	}
}
