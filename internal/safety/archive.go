package safety

import (
	"context"
	"fmt"
)

type ArchiveLimits struct {
	MaxEntries               int
	MaxExpandedBytes         int64
	MaxEntryBytes            int64
	MaxCompressionRatio      float64
	MaxDepth                 int
	MaxPathBytes             int
	ReservationHeadroomBytes int64
}

func DefaultArchiveLimits() ArchiveLimits {
	return ArchiveLimits{
		MaxEntries:               250_000,
		MaxExpandedBytes:         8 << 30,
		MaxEntryBytes:            512 << 20,
		MaxCompressionRatio:      250,
		MaxDepth:                 32,
		MaxPathBytes:             4096,
		ReservationHeadroomBytes: 64 << 20,
	}
}

func (l ArchiveLimits) Valid() bool {
	return l.MaxEntries > 0 && l.MaxExpandedBytes > 0 && l.MaxEntryBytes > 0 &&
		l.MaxEntryBytes <= l.MaxExpandedBytes && l.MaxCompressionRatio >= 1 &&
		l.MaxDepth > 0 && l.MaxPathBytes > 0 && l.ReservationHeadroomBytes >= 0
}

type ArchiveExtractRequest struct {
	Sources       []string
	Destination   string
	ExpectedRoots []string
	Limits        ArchiveLimits
}

type ArchiveInspectRequest struct {
	Sources       []string
	ExpectedRoots []string
	Limits        ArchiveLimits
}

type ArchiveSourceEvidence struct {
	Name            string `json:"name"`
	SHA256          string `json:"sha256"`
	CompressedBytes int64  `json:"compressed_bytes"`
}

type ArchiveExtractResult struct {
	Destination   string                  `json:"destination"`
	Files         int                     `json:"files"`
	Directories   int                     `json:"directories"`
	ExpandedBytes int64                   `json:"expanded_bytes"`
	Sources       []ArchiveSourceEvidence `json:"sources"`
}

type archiveEntryKind string

const (
	archiveRegularFile archiveEntryKind = "file"
	archiveDirectory   archiveEntryKind = "directory"
)

type archiveEntry struct {
	Name string
	Kind archiveEntryKind
	Size int64
}

type archiveSourcePlan struct {
	Path            string
	SHA256          string
	CompressedBytes int64
	Entries         []archiveEntry
}

type archivePlan struct {
	Sources       []archiveSourcePlan
	Files         int
	Directories   int
	ExpandedBytes int64
}

func ExtractTarGzip(ctx context.Context, request ArchiveExtractRequest) (ArchiveExtractResult, error) {
	if ctx == nil || request.Destination == "" || len(request.Sources) == 0 || !request.Limits.Valid() {
		return ArchiveExtractResult{}, &Error{Kind: ErrorInvalidInput, Operation: OperationExtract}
	}
	plan, err := inspectTarGzipSources(ctx, request)
	if err != nil {
		return ArchiveExtractResult{}, err
	}
	return extractTarGzipPlan(ctx, request, plan)
}

func InspectTarGzip(ctx context.Context, request ArchiveInspectRequest) (ArchiveExtractResult, error) {
	if ctx == nil || len(request.Sources) == 0 || !request.Limits.Valid() {
		return ArchiveExtractResult{}, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
	plan, err := inspectTarGzipSources(ctx, ArchiveExtractRequest{
		Sources: request.Sources, ExpectedRoots: request.ExpectedRoots, Limits: request.Limits,
	})
	if err != nil {
		return ArchiveExtractResult{}, err
	}
	return archiveResult("", plan), nil
}

func archiveLocation(sourceIndex, entryIndex int) string {
	return fmt.Sprintf("archive[%d].entry[%d]", sourceIndex, entryIndex)
}
