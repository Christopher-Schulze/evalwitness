package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestDisabledLoggerWritesNothingAndNeverFails(t *testing.T) {
	// An empty path means auditing is off. Every method still has to be safe to
	// call, because the caller does not branch on it.
	l, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if l.Enabled() {
		t.Fatal("logger with an empty path reports itself enabled")
	}
	if err := l.Write(Entry{Provider: "p"}); err != nil {
		t.Fatalf("disabled logger write: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("closing a disabled logger returned %v", err)
	}
}

func TestNilLoggerIsSafe(t *testing.T) {
	// Callers hold a *Logger that may be nil when auditing was never configured.
	// A nil dereference here would take down a run over a feature that is off.
	var l *Logger
	if l.Enabled() {
		t.Fatal("nil logger reports itself enabled")
	}
	if err := l.Write(Entry{Provider: "p"}); err != nil {
		t.Fatalf("nil logger write: %v", err)
	}
}

func TestWriteProducesOneJSONObjectPerLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "audit.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	cost := 0.25
	if err := l.Write(Entry{Provider: "deepseek", Model: "m", CriterionID: "generic", InputTokens: 10, EstCostUSD: &cost}); err != nil {
		t.Fatal(err)
	}
	if err := l.Write(Entry{Provider: "deepseek", Model: "m", CriterionID: "generic", CacheHit: true, CacheNamespace: "evalwitness"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("New did not create the parent directory: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close audit file: %v", err)
		}
	}()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("line is not a JSON object: %v", err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("wrote %d lines, want 2", len(entries))
	}
	if entries[0].EstCostUSD == nil || *entries[0].EstCostUSD != 0.25 {
		t.Fatalf("cost = %v, want 0.25", entries[0].EstCostUSD)
	}
	if !entries[1].CacheHit {
		t.Fatal("cache hit did not survive the round trip")
	}
	if entries[1].CacheNamespace != "evalwitness" {
		t.Fatalf("cache namespace = %q", entries[1].CacheNamespace)
	}
	// A zero timestamp is filled in, because an audit line without a time is
	// not an audit line.
	for i, e := range entries {
		if e.TS == 0 {
			t.Fatalf("entry %d has no timestamp", i)
		}
	}
}

func TestExplicitTimestampIsPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, _ := New(path)
	if err := l.Write(Entry{TS: 1700000000, Provider: "p"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e.TS != 1700000000 {
		t.Fatalf("ts = %d, want the caller's value preserved", e.TS)
	}
}

func TestAppendsRatherThanTruncating(t *testing.T) {
	// A second run must not erase the first run's audit trail.
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	first, _ := New(path)
	if err := first.Write(Entry{Provider: "run-one"}); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, _ := New(path)
	if err := second.Write(Entry{Provider: "run-two"}); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		lines++
	}
	if lines != 2 {
		t.Fatalf("%d lines after two runs, want 2; the second run truncated the first", lines)
	}
}

func TestFileIsNotWorldReadable(t *testing.T) {
	// The log records provider, model and token counts for real runs. It is
	// created 0600 on purpose.
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Errorf("close audit logger: %v", err)
		}
	}()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 600", perm)
	}
}

func TestLoggerSecuresNewParentAndExistingFile(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "private", "audit")
	path := filepath.Join(parent, "audit.jsonl")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	logger, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != safety.SensitiveFileMode {
		t.Fatalf("file mode = %o", info.Mode().Perm())
	}

	newParent := filepath.Join(root, "new", "nested")
	logger, err = New(filepath.Join(newParent, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(newParent)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != safety.SensitiveDirectoryMode {
		t.Fatalf("directory mode = %o", info.Mode().Perm())
	}
}

func TestConcurrentWritesDoNotInterleave(t *testing.T) {
	// Pair evaluation runs concurrently, so writes race by construction. A torn
	// line would corrupt the record of the run it exists to document.
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	const writers, each = 8, 50
	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for range each {
				if err := l.Write(Entry{Provider: "p", Model: "m", CriterionID: "generic", InputTokens: w}); err != nil {
					t.Errorf("concurrent audit write: %v", err)
				}
			}
		})
	}
	wg.Wait()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close audit file: %v", err)
		}
	}()
	lines := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("line %d is torn: %v", lines+1, err)
		}
		lines++
	}
	if lines != writers*each {
		t.Fatalf("%d lines, want %d", lines, writers*each)
	}
}

func TestWriteFailureIsObservable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	logger, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Provider: "after-close"}); err == nil {
		t.Fatal("write to a closed audit file reported success")
	}
}
