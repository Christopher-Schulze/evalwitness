package safety

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzArchiveHeaderName(f *testing.F) {
	for _, seed := range []string{"root/file.json", "../escape", `/absolute`, `C:\escape`, "e\u0301/file", ".evalwitness-reservation"} {
		f.Add(seed, false)
	}
	f.Fuzz(func(t *testing.T, raw string, directory bool) {
		kind := archiveRegularFile
		if directory {
			kind = archiveDirectory
		}
		limits := DefaultArchiveLimits()
		normalized, err := normalizeArchiveName(raw, kind, limits, "fuzz")
		if err != nil {
			return
		}
		want := raw
		if directory {
			want = strings.TrimSuffix(want, "/")
		}
		if normalized != want || !utf8.ValidString(normalized) || filepath.IsAbs(normalized) || strings.Contains(normalized, "\\") {
			t.Fatalf("unsafe normalization %q -> %q", raw, normalized)
		}
	})
}

func FuzzRouteNamespace(f *testing.F) {
	for _, seed := range [][2]string{{"openai", "model"}, {"../provider", "../../model"}, {"provider/slash", "model\\slash"}, {"", "model"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, provider, model string) {
		namespace, err := NewRouteNamespace(provider, model)
		if err != nil {
			return
		}
		if !namespace.Valid() || !IsSafeNamespaceID(namespace.ID) {
			t.Fatalf("invalid accepted namespace: %+v", namespace)
		}
		parts := strings.Split(filepath.ToSlash(namespace.Directory()), "/")
		if len(parts) != 2 || parts[0] != "routes" || parts[1] != namespace.ID {
			t.Fatalf("unsafe namespace directory %q", namespace.Directory())
		}
	})
}

func FuzzPathContainment(f *testing.F) {
	for _, seed := range [][2]string{{"/tmp/root", "/tmp/root/child"}, {"/tmp/root", "/tmp/root-other"}, {".", "../escape"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, parent, child string) {
		contained := containsOrEqual(parent, child)
		if !contained {
			return
		}
		relative, err := filepath.Rel(comparisonPath(parent), comparisonPath(child))
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Fatalf("escaping child reported contained: parent=%q child=%q relative=%q err=%v", parent, child, relative, err)
		}
	})
}

func FuzzSecretRedaction(f *testing.F) {
	for _, seed := range []string{
		"safe", "Authorization: Bearer abcdefghijklmnopqrstuvwxyz", "api_key=tiny",
		"-----BEGIN PRIVATE KEY-----", "Cookie: session=value",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		output := RedactSecretPatterns(input)
		if len(output) > len(input)*2+64 {
			t.Fatalf("redaction expanded %d bytes to %d", len(input), len(output))
		}
		for _, match := range FindSecretPatterns(input) {
			if match.Start < 0 || match.End < match.Start || match.End > len(input) {
				t.Fatalf("invalid match bounds: %+v for %d bytes", match, len(input))
			}
		}
	})
}
