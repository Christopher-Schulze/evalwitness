package safety

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type archiveTestEntry struct {
	name       string
	kind       byte
	body       []byte
	link       string
	paxRecords map[string]string
	mode       int64
}

func testArchiveLimits() ArchiveLimits {
	limits := DefaultArchiveLimits()
	limits.ReservationHeadroomBytes = 1024
	limits.MaxCompressionRatio = 1000
	return limits
}

func writeTestTarGzip(t *testing.T, entries []archiveTestEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.tar.gz")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		mode := entry.mode
		if mode == 0 {
			mode = 0o600
		}
		header := &tar.Header{
			Name:       entry.name,
			Typeflag:   entry.kind,
			Size:       int64(len(entry.body)),
			Mode:       mode,
			Linkname:   entry.link,
			PAXRecords: entry.paxRecords,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(entry.body) > 0 {
			if _, err := tarWriter.Write(entry.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractTarGzipValidatesThenPublishesMultipleArchives(t *testing.T) {
	terminal := writeTestTarGzip(t, []archiveTestEntry{
		{name: "terminal_trajs/", kind: tar.TypeDir},
		{name: "terminal_trajs/task.json", kind: tar.TypeReg, body: []byte(`{"task":1}`)},
	})
	swebench := writeTestTarGzip(t, []archiveTestEntry{
		{name: "swebench_verified_trajs/", kind: tar.TypeDir},
		{name: "swebench_verified_trajs/run.json", kind: tar.TypeReg, body: []byte(`{"run":2}`)},
	})
	destination := filepath.Join(t.TempDir(), "trajectories")
	result, err := ExtractTarGzip(context.Background(), ArchiveExtractRequest{
		Sources:       []string{terminal, swebench},
		Destination:   destination,
		ExpectedRoots: []string{"terminal_trajs", "swebench_verified_trajs"},
		Limits:        testArchiveLimits(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 2 || result.Directories != 2 || result.ExpandedBytes != 19 || len(result.Sources) != 2 {
		t.Fatalf("result = %+v", result)
	}
	for relative, want := range map[string]string{
		"terminal_trajs/task.json":         `{"task":1}`,
		"swebench_verified_trajs/run.json": `{"run":2}`,
	} {
		path := filepath.Join(destination, filepath.FromSlash(relative))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != want {
			t.Fatalf("%s = %q", relative, raw)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != SensitiveFileMode {
			t.Fatalf("%s mode = %o", relative, info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(destination, ".evalwitness-reservation")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reservation published: %v", err)
	}
}

func TestExtractTarGzipRejectsHostileNamesAndTypesBeforePublication(t *testing.T) {
	tests := []struct {
		name  string
		entry archiveTestEntry
		kind  ErrorKind
	}{
		{"parent traversal", archiveTestEntry{name: "../escape", kind: tar.TypeReg}, ErrorContainmentViolation},
		{"absolute", archiveTestEntry{name: "/escape", kind: tar.TypeReg}, ErrorContainmentViolation},
		{"windows volume", archiveTestEntry{name: "C:/escape", kind: tar.TypeReg}, ErrorContainmentViolation},
		{"backslash", archiveTestEntry{name: `root\..\escape`, kind: tar.TypeReg}, ErrorInvalidInput},
		{"repeated separator", archiveTestEntry{name: "root//escape", kind: tar.TypeReg}, ErrorContainmentViolation},
		{"dot segment", archiveTestEntry{name: "root/./escape", kind: tar.TypeReg}, ErrorContainmentViolation},
		{"non NFC", archiveTestEntry{name: "root/cafe\u0301", kind: tar.TypeReg}, ErrorContainmentViolation},
		{"reserved internal", archiveTestEntry{name: ".evalwitness-reservation", kind: tar.TypeReg}, ErrorResourceLimit},
		{"symlink", archiveTestEntry{name: "root/link", kind: tar.TypeSymlink, link: "outside"}, ErrorUnsupportedFileType},
		{"hard link", archiveTestEntry{name: "root/link", kind: tar.TypeLink, link: "root/file"}, ErrorUnsupportedFileType},
		{"fifo", archiveTestEntry{name: "root/fifo", kind: tar.TypeFifo}, ErrorUnsupportedFileType},
		{"character device", archiveTestEntry{name: "root/device", kind: tar.TypeChar}, ErrorUnsupportedFileType},
		{"xattrs", archiveTestEntry{name: "root/file", kind: tar.TypeReg, paxRecords: map[string]string{"SCHILY.xattr.user.secret": "value"}}, ErrorUnsupportedFileType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := writeTestTarGzip(t, []archiveTestEntry{test.entry})
			parent := t.TempDir()
			sentinel := filepath.Join(parent, "sentinel")
			if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(parent, "destination")
			_, err := ExtractTarGzip(context.Background(), ArchiveExtractRequest{
				Sources: []string{source}, Destination: destination, Limits: testArchiveLimits(),
			})
			if !IsKind(err, test.kind) {
				t.Fatalf("error = %T %v, want %s", err, err, test.kind)
			}
			if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("destination was published: %v", statErr)
			}
			if raw, readErr := os.ReadFile(sentinel); readErr != nil || string(raw) != "unchanged" {
				t.Fatalf("sentinel changed: %q %v", raw, readErr)
			}
		})
	}
}

func TestExtractTarGzipRejectsNameCollisions(t *testing.T) {
	tests := []struct {
		name    string
		entries []archiveTestEntry
	}{
		{"exact duplicate", []archiveTestEntry{{name: "root/file", kind: tar.TypeReg}, {name: "root/file", kind: tar.TypeReg}}},
		{"case fold", []archiveTestEntry{{name: "root/File", kind: tar.TypeReg}, {name: "root/file", kind: tar.TypeReg}}},
		{"unicode case fold", []archiveTestEntry{{name: "root/Straße", kind: tar.TypeReg}, {name: "root/STRASSE", kind: tar.TypeReg}}},
		{"file before child", []archiveTestEntry{{name: "root/node", kind: tar.TypeReg}, {name: "root/node/child", kind: tar.TypeReg}}},
		{"child before file", []archiveTestEntry{{name: "root/node/child", kind: tar.TypeReg}, {name: "root/node", kind: tar.TypeReg}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := writeTestTarGzip(t, test.entries)
			_, err := ExtractTarGzip(context.Background(), ArchiveExtractRequest{
				Sources: []string{source}, Destination: filepath.Join(t.TempDir(), "destination"), Limits: testArchiveLimits(),
			})
			if !IsKind(err, ErrorNameCollision) {
				t.Fatalf("error = %T %v, want name collision", err, err)
			}
		})
	}
}

func TestExtractTarGzipEnforcesEveryResourceLimit(t *testing.T) {
	tests := []struct {
		name    string
		entries []archiveTestEntry
		mutate  func(*ArchiveLimits)
	}{
		{"entry count", []archiveTestEntry{{name: "root/a", kind: tar.TypeReg}, {name: "root/b", kind: tar.TypeReg}}, func(l *ArchiveLimits) { l.MaxEntries = 1 }},
		{"entry bytes", []archiveTestEntry{{name: "root/a", kind: tar.TypeReg, body: []byte("12345")}}, func(l *ArchiveLimits) { l.MaxEntryBytes = 4 }},
		{"total bytes", []archiveTestEntry{{name: "root/a", kind: tar.TypeReg, body: []byte("123")}, {name: "root/b", kind: tar.TypeReg, body: []byte("456")}}, func(l *ArchiveLimits) { l.MaxExpandedBytes = 5; l.MaxEntryBytes = 5 }},
		{"nesting", []archiveTestEntry{{name: "a/b/c", kind: tar.TypeReg}}, func(l *ArchiveLimits) { l.MaxDepth = 2 }},
		{"path bytes", []archiveTestEntry{{name: "root/long", kind: tar.TypeReg}}, func(l *ArchiveLimits) { l.MaxPathBytes = 5 }},
		{"compression ratio", []archiveTestEntry{{name: "root/zeros", kind: tar.TypeReg, body: bytes.Repeat([]byte{0}, 1024*1024)}}, func(l *ArchiveLimits) { l.MaxCompressionRatio = 2 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := writeTestTarGzip(t, test.entries)
			limits := testArchiveLimits()
			test.mutate(&limits)
			_, err := ExtractTarGzip(context.Background(), ArchiveExtractRequest{
				Sources: []string{source}, Destination: filepath.Join(t.TempDir(), "destination"), Limits: limits,
			})
			if !IsKind(err, ErrorResourceLimit) {
				t.Fatalf("error = %T %v, want resource limit", err, err)
			}
		})
	}
}

func TestExtractTarGzipRejectsUnexpectedRootTrailingDataAndExistingDestination(t *testing.T) {
	valid := writeTestTarGzip(t, []archiveTestEntry{{name: "root/file", kind: tar.TypeReg, body: []byte("data")}})
	_, err := ExtractTarGzip(context.Background(), ArchiveExtractRequest{
		Sources: []string{valid}, Destination: filepath.Join(t.TempDir(), "unexpected-root"),
		ExpectedRoots: []string{"allowed"}, Limits: testArchiveLimits(),
	})
	if !IsKind(err, ErrorArtifactPolicyViolation) {
		t.Fatalf("unexpected root error = %T %v", err, err)
	}

	withTrailing := filepath.Join(t.TempDir(), "trailing.tar.gz")
	raw, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(withTrailing, append(raw, []byte("trailing")...), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ExtractTarGzip(context.Background(), ArchiveExtractRequest{
		Sources: []string{withTrailing}, Destination: filepath.Join(t.TempDir(), "trailing"), Limits: testArchiveLimits(),
	})
	if err == nil {
		t.Fatal("archive with trailing data was accepted")
	}

	destination := filepath.Join(t.TempDir(), "existing")
	if err := os.MkdirAll(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(destination, "sentinel")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ExtractTarGzip(context.Background(), ArchiveExtractRequest{
		Sources: []string{valid}, Destination: destination, Limits: testArchiveLimits(),
	})
	if !IsKind(err, ErrorArtifactPolicyViolation) {
		t.Fatalf("existing destination error = %T %v", err, err)
	}
	if raw, readErr := os.ReadFile(sentinel); readErr != nil || string(raw) != "unchanged" {
		t.Fatalf("existing destination changed: %q %v", raw, readErr)
	}
}

func TestExtractTarGzipHonorsCancellationAndRejectsNonRegularSource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	valid := writeTestTarGzip(t, []archiveTestEntry{{name: "root/file", kind: tar.TypeReg}})
	_, err := ExtractTarGzip(ctx, ArchiveExtractRequest{
		Sources: []string{valid}, Destination: filepath.Join(t.TempDir(), "cancelled"), Limits: testArchiveLimits(),
	})
	if err == nil {
		t.Fatal("cancelled extraction succeeded")
	}

	_, err = ExtractTarGzip(context.Background(), ArchiveExtractRequest{
		Sources: []string{t.TempDir()}, Destination: filepath.Join(t.TempDir(), "directory-source"), Limits: testArchiveLimits(),
	})
	if err == nil {
		t.Fatal("directory source was accepted")
	}
}

func TestDiskReservationReleasesCapacityAsEntriesCommit(t *testing.T) {
	directory := t.TempDir()
	reservation, err := newDiskReservation(directory, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := reservation.file.Stat(); err != nil || info.Size() != 4096 {
		t.Fatalf("reserved size = %v, %v", info, err)
	}
	if err := reservation.release(1024); err != nil {
		t.Fatal(err)
	}
	if info, err := reservation.file.Stat(); err != nil || info.Size() != 3072 {
		t.Fatalf("released size = %v, %v", info, err)
	}
	if err := reservation.release(4096); !IsKind(err, ErrorResourceLimit) {
		t.Fatalf("over-release error = %T %v", err, err)
	}
	path := reservation.path
	if err := reservation.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reservation remained: %v", err)
	}
}

func TestWriteTestTarGzipRejectsTruncatedDeclaredBody(t *testing.T) {
	valid := writeTestTarGzip(t, []archiveTestEntry{{name: "root/file", kind: tar.TypeReg, body: []byte("complete")}})
	raw, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 16 {
		t.Fatalf("archive unexpectedly short: %d", len(raw))
	}
	path := filepath.Join(t.TempDir(), "truncated.tar.gz")
	if err := os.WriteFile(path, raw[:len(raw)-12], 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ExtractTarGzip(context.Background(), ArchiveExtractRequest{
		Sources: []string{path}, Destination: filepath.Join(t.TempDir(), "truncated"), Limits: testArchiveLimits(),
	})
	if err == nil || strings.Contains(err.Error(), "short") {
		t.Fatalf("truncated archive error = %v", err)
	}
}
