package safety

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultUntrustedInputLimitsCoverEveryFutureBoundary(t *testing.T) {
	for _, kind := range []UntrustedInputKind{
		InputProtocolAdapter, InputTrace, InputAttribution, InputCapsule, InputPolicy, InputStaticReport,
	} {
		limits, err := DefaultUntrustedInputLimits(kind)
		if err != nil || !limits.Valid() {
			t.Fatalf("kind %q limits = %+v, %v", kind, limits, err)
		}
	}
	if _, err := DefaultUntrustedInputLimits("unknown"); !IsKind(err, ErrorInvalidInput) {
		t.Fatalf("unknown kind error = %T %v", err, err)
	}
}

func TestValidateUntrustedJSONEnforcesEveryStructuralBound(t *testing.T) {
	base := UntrustedInputLimits{
		MaxBytes: 1024, MaxDepth: 4, MaxTotalNodes: 10, MaxStringBytes: 8,
		MaxArrayItems: 3, MaxObjectFields: 2, MaxMarkupBytes: 32, MaxLinks: 2,
	}
	tests := []struct {
		name string
		raw  string
		edit func(*UntrustedInputLimits)
		kind ErrorKind
	}{
		{name: "valid", raw: `{"a":[1,true,"ok"]}`},
		{name: "bytes", raw: `{"a":1}`, edit: func(l *UntrustedInputLimits) { l.MaxBytes = 2 }, kind: ErrorResourceLimit},
		{name: "depth", raw: `[[[[[1]]]]]`, kind: ErrorResourceLimit},
		{name: "nodes", raw: `[1,2,3]`, edit: func(l *UntrustedInputLimits) { l.MaxTotalNodes = 3 }, kind: ErrorResourceLimit},
		{name: "string", raw: `"123456789"`, kind: ErrorResourceLimit},
		{name: "array", raw: `[1,2,3,4]`, kind: ErrorResourceLimit},
		{name: "fields", raw: `{"a":1,"b":2,"c":3}`, kind: ErrorResourceLimit},
		{name: "trailing", raw: `{} {}`, kind: ErrorInvalidInput},
		{name: "malformed", raw: `{`, kind: ErrorInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limits := base
			if test.edit != nil {
				test.edit(&limits)
			}
			err := ValidateUntrustedJSON([]byte(test.raw), limits)
			if test.kind == "" && err != nil {
				t.Fatal(err)
			}
			if test.kind != "" && !IsKind(err, test.kind) {
				t.Fatalf("error = %T %v, want %s", err, err, test.kind)
			}
		})
	}
}

func TestReadAndValidateUntrustedJSONRejectsSymlinkAndOversize(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input.json")
	if err := os.WriteFile(path, []byte(`{"ok":true}`), SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	limits, err := DefaultUntrustedInputLimits(InputPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAndValidateUntrustedJSON(path, limits); err != nil {
		t.Fatal(err)
	}
	limits.MaxBytes = 2
	if _, err := ReadAndValidateUntrustedJSON(path, limits); !IsKind(err, ErrorResourceLimit) {
		t.Fatalf("oversize error = %T %v", err, err)
	}
	link := filepath.Join(root, "link.json")
	if err := os.Symlink(path, link); err == nil {
		if _, err := ReadAndValidateUntrustedJSON(link, limits); !IsKind(err, ErrorUnsupportedFileType) {
			t.Fatalf("symlink error = %T %v", err, err)
		}
	}
}

func TestUntrustedInputCannotSelectOperatorControls(t *testing.T) {
	if err := RejectUntrustedControlSelection(UntrustedControlSelection{}); err != nil {
		t.Fatal(err)
	}
	controls := []UntrustedControlSelection{
		{Command: []string{"tool"}}, {Environment: map[string]string{"KEY": "value"}},
		{WorkingDirectory: "/tmp"}, {NetworkDestination: "https://example.invalid"},
		{OutputPath: "result.json"}, {LiveMode: true},
	}
	for index, control := range controls {
		if err := RejectUntrustedControlSelection(control); !IsKind(err, ErrorUntrustedControl) {
			t.Fatalf("control %d error = %T %v", index, err, err)
		}
	}
}

func TestMarkupAndOfflineLinksAreBoundedAndConfined(t *testing.T) {
	limits, err := DefaultUntrustedInputLimits(InputStaticReport)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateUntrustedMarkup("<b>escaped later</b>", limits); err != nil {
		t.Fatal(err)
	}
	limits.MaxMarkupBytes = 2
	if err := ValidateUntrustedMarkup("long", limits); !IsKind(err, ErrorResourceLimit) {
		t.Fatalf("markup error = %T %v", err, err)
	}
	for _, link := range []string{"index.html", "profiles/run.json#scope", "#finding-1"} {
		if err := ValidateOfflineLink(link); err != nil {
			t.Fatalf("safe link %q: %v", link, err)
		}
	}
	for _, link := range []string{"https://example.invalid", "//example.invalid/x", "/etc/passwd", "../secret", `..\secret`, "file:///etc/passwd", "page.html?q=secret"} {
		if err := ValidateOfflineLink(link); err == nil {
			t.Fatalf("unsafe link %q accepted", link)
		}
	}
	if err := ValidateOfflineLink(strings.Repeat("a", 4)); err != nil {
		t.Fatal(err)
	}
}
