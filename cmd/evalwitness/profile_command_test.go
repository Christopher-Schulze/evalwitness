package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileCommandBuildVerifyDiffPolicyRender(t *testing.T) {
	dir := t.TempDir()
	pPath := filepath.Join(dir, "p.json")
	out := runCapture(t, func() int {
		return runProfile([]string{"build", "--identity", "smoke", "--route", "r1",
			"--dim", "calibration:measured:0.12:terminal:E1:capsule:sha256-abc:100:task"})
	})
	if out.code != 0 {
		t.Fatalf("build exit %d: %s", out.code, out.stderr)
	}
	if err := os.WriteFile(pPath, []byte(out.stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	v := runCapture(t, func() int { return runProfile([]string{"verify", "--in", pPath}) })
	if v.code != 0 {
		t.Fatalf("verify exit %d: %s", v.code, v.stderr)
	}
	d := runCapture(t, func() int { return runProfile([]string{"diff", "--a", pPath, "--b", pPath, "--format", "text"}) })
	if d.code != 0 || !contains(d.stdout, "compatible=true") {
		t.Fatalf("diff exit %d out %s err %s", d.code, d.stdout, d.stderr)
	}
	polPath := filepath.Join(dir, "pol.json")
	if err := os.WriteFile(polPath, []byte(`{"version":"v1","requirements":{"calibration":"measured"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pl := runCapture(t, func() int { return runProfile([]string{"policy", "--policy", polPath, "--in", pPath}) })
	if pl.code != 0 || !contains(pl.stdout, `"pass": true`) {
		t.Fatalf("policy exit %d out %s err %s", pl.code, pl.stdout, pl.stderr)
	}
	r := runCapture(t, func() int { return runProfile([]string{"render", "--in", pPath, "--format", "text"}) })
	if r.code != 0 || !contains(r.stdout, "Profile smoke 1 dims digest") {
		t.Fatalf("render exit %d out %s err %s", r.code, r.stdout, r.stderr)
	}
}

func TestProfileCommandPolicyFailRendersFails(t *testing.T) {
	dir := t.TempDir()
	pPath := filepath.Join(dir, "p.json")
	built := runCapture(t, func() int {
		return runProfile([]string{"build", "--identity", "x", "--route", "r",
			"--dim", "a:failed:1:s:E1:c:10:task"})
	})
	if built.code != 0 {
		t.Fatalf("build %d %s", built.code, built.stderr)
	}
	if err := os.WriteFile(pPath, []byte(built.stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	polPath := filepath.Join(dir, "pol.json")
	if err := os.WriteFile(polPath, []byte(`{"version":"v1","requirements":{"a":"measured"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pl := runCapture(t, func() int { return runProfile([]string{"policy", "--policy", polPath, "--in", pPath}) })
	if pl.code != 1 || !contains(pl.stdout, `"pass": false`) || !contains(pl.stdout, "want measured got failed") {
		t.Fatalf("policy must fail with rendered reason: code=%d out=%s", pl.code, pl.stdout)
	}
}

func TestParseDimColonExpr(t *testing.T) {
	d, err := parseDim("id:measured:0.5:scope:E1:capsule:sha256-deadbeef:42:task")
	if err != nil {
		t.Fatalf("parseDim %v", err)
	}
	if d.CapsuleExpr != "capsule:sha256-deadbeef" || d.Denominator != 42 || d.SampleUnit != "task" || d.ID != "id" || d.EvidenceLevel != "E1" {
		t.Fatalf("parsed %+v", d)
	}
	if _, err := parseDim("toofew"); err == nil {
		t.Fatal("expected error on short spec")
	}
}

type captured struct {
	code   int
	stdout string
	stderr string
}

// runCapture runs fn with os.Stdout/os.Stderr redirected through pipes.
func runCapture(t *testing.T, fn func() int) captured {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	code := fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}
	var outBuf, errBuf bytes.Buffer
	if _, err := outBuf.ReadFrom(outR); err != nil {
		t.Fatal(err)
	}
	if _, err := errBuf.ReadFrom(errR); err != nil {
		t.Fatal(err)
	}
	return captured{code: code, stdout: outBuf.String(), stderr: errBuf.String()}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
