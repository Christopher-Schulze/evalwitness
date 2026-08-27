package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	LegacyCacheCensusSchemaVersion = "evalwitness.legacy-cache-census.v1"
	legacyCacheInventoryAlgorithm  = "evalwitness.legacy-cache-inventory.v1"
	legacyCacheScopeUpperBound     = "namespace_upper_bound_not_request_mapped"
	legacyCacheMaxFiles            = 100_000
	legacyCacheMaxTotalBytes       = int64(8 << 30)
)

var (
	legacyResponsePathPattern     = regexp.MustCompile(`^([a-z0-9][a-z0-9._-]*)/([0-9a-f]{2})/([0-9a-f]{64})\.json$`)
	legacyCapabilityPattern       = regexp.MustCompile(`^caps/[A-Za-z0-9._-]+\.json$`)
	legacyCensusMissingIdentities = []string{
		"analysis_identity",
		"binary_identity",
		"capability_attestation_identity",
		"checkpoint_assertion_identity",
		"collection_clock",
		"dataset_identity",
		"finish_state_identity",
		"parsed_payload_identity",
		"parser_contract_identity",
		"provider_request_identity",
		"raw_response_identity",
		"request_identity",
		"request_lineage_identity",
		"response_body_identity",
		"response_evidence_identity",
		"response_record_identity",
		"route_attestation",
		"route_identity",
		"sampling_slot_identity",
		"score_evidence_identity",
		"served_model_identity",
		"source_tree_identity",
		"usage_identity",
	}
)

type LegacyProviderCensus struct {
	ProviderID string `json:"provider_id"`
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
}

type LegacyPublishedNamespace struct {
	ProviderID      string `json:"provider_id"`
	Files           int    `json:"files"`
	Bytes           int64  `json:"bytes"`
	ScopeStatus     string `json:"scope_status"`
	ExactRequestMap bool   `json:"exact_request_map"`
}

type LegacyCacheCensus struct {
	SchemaVersion             string                   `json:"schema_version"`
	InventoryAlgorithm        string                   `json:"inventory_algorithm"`
	ResponseSchemaVersion     int                      `json:"response_schema_version"`
	TotalFiles                int                      `json:"total_files"`
	TotalBytes                int64                    `json:"total_bytes"`
	ResponseFiles             int                      `json:"response_files"`
	ResponseBytes             int64                    `json:"response_bytes"`
	CapabilityFiles           int                      `json:"capability_files"`
	CapabilityBytes           int64                    `json:"capability_bytes"`
	OperationalFiles          int                      `json:"operational_files"`
	OperationalBytes          int64                    `json:"operational_bytes"`
	Providers                 []LegacyProviderCensus   `json:"providers"`
	PublishedNamespace        LegacyPublishedNamespace `json:"published_namespace"`
	ScientificInventoryDigest string                   `json:"scientific_inventory_digest"`
	OperationalMetadataDigest string                   `json:"operational_metadata_digest"`
	ExactAdmissibleEntries    int                      `json:"exact_admissible_entries"`
	LegacyOnlyEntries         int                      `json:"legacy_only_entries"`
	MissingIdentities         []string                 `json:"missing_identities"`
	OperationalContentRead    bool                     `json:"operational_content_read"`
	SensitiveContentEmitted   bool                     `json:"sensitive_content_emitted"`
	ReadOnly                  bool                     `json:"read_only"`
	ProviderCalls             int                      `json:"provider_calls"`
	Digest                    string                   `json:"digest"`
}

type legacyInventoryRecord struct {
	Class  string `json:"class"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type legacyOperationalRecord struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// LegacyCacheMissingIdentities returns the exact evidence identities unavailable
// from a schema-1 response corpus.
func LegacyCacheMissingIdentities() []string {
	return slices.Clone(legacyCensusMissingIdentities)
}

func CensusLegacyCache(rootPath, publishedProvider string) (result LegacyCacheCensus, returnErr error) {
	if strings.TrimSpace(rootPath) == "" || !validLegacyProviderID(publishedProvider) {
		return LegacyCacheCensus{}, errors.New("legacy cache census requires a root and safe published provider")
	}
	rootInfo, err := os.Lstat(rootPath)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return LegacyCacheCensus{}, errors.Join(err, errors.New("legacy cache census root is not a real directory"))
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return LegacyCacheCensus{}, err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil && returnErr == nil {
			returnErr = closeErr
		}
	}()

	providerCounts := make(map[string]LegacyProviderCensus)
	scientificRecords := make([]legacyInventoryRecord, 0)
	operationalRecords := make([]legacyOperationalRecord, 0)
	census := LegacyCacheCensus{
		SchemaVersion: LegacyCacheCensusSchemaVersion, InventoryAlgorithm: legacyCacheInventoryAlgorithm,
		ResponseSchemaVersion: 1, ExactAdmissibleEntries: 0,
		MissingIdentities:      slices.Clone(legacyCensusMissingIdentities),
		OperationalContentRead: false, SensitiveContentEmitted: false, ReadOnly: true, ProviderCalls: 0,
	}
	err = fs.WalkDir(root.FS(), ".", func(relative string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("legacy cache census rejects symlink %q", filepath.ToSlash(relative))
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("legacy cache census rejects non-regular file %q", filepath.ToSlash(relative))
		}
		if census.TotalFiles >= legacyCacheMaxFiles || info.Size() < 0 || info.Size() > maxEntryBytes ||
			info.Size() > legacyCacheMaxTotalBytes-census.TotalBytes {
			return errors.New("legacy cache census exceeds its resource bounds")
		}
		path := filepath.ToSlash(relative)
		census.TotalFiles++
		census.TotalBytes += info.Size()
		if matches := legacyResponsePathPattern.FindStringSubmatch(path); matches != nil {
			if matches[2] != matches[3][:2] {
				return fmt.Errorf("legacy response path %q has a mismatched shard", path)
			}
			raw, err := readLegacyCensusFile(root, relative, info)
			if err != nil {
				return err
			}
			var legacy LegacyEntry
			if err := json.Unmarshal(raw, &legacy); err != nil || legacy.SchemaVersion != 1 {
				return fmt.Errorf("legacy response %q is not schema 1", path)
			}
			census.ResponseFiles++
			census.ResponseBytes += int64(len(raw))
			provider := providerCounts[matches[1]]
			provider.ProviderID = matches[1]
			provider.Files++
			provider.Bytes += int64(len(raw))
			providerCounts[matches[1]] = provider
			scientificRecords = append(scientificRecords, legacyInventoryRecord{
				Class: "response_schema1", Path: path, SHA256: protocol.DigestBytes(raw), Bytes: int64(len(raw)),
			})
			return nil
		}
		if legacyCapabilityPattern.MatchString(path) {
			raw, err := readLegacyCensusFile(root, relative, info)
			if err != nil {
				return err
			}
			census.CapabilityFiles++
			census.CapabilityBytes += int64(len(raw))
			scientificRecords = append(scientificRecords, legacyInventoryRecord{
				Class: "capability_diagnostic", Path: path, SHA256: protocol.DigestBytes(raw), Bytes: int64(len(raw)),
			})
			return nil
		}
		if strings.HasPrefix(path, "supervisor/") {
			if err := observeLegacyCensusFileMetadata(root, relative, info); err != nil {
				return err
			}
			census.OperationalFiles++
			census.OperationalBytes += info.Size()
			operationalRecords = append(operationalRecords, legacyOperationalRecord{Path: path, Bytes: info.Size()})
			return nil
		}
		return fmt.Errorf("legacy cache census rejects unexpected file %q", path)
	})
	if err != nil {
		return LegacyCacheCensus{}, err
	}
	for _, providerID := range sortedLegacyProviderIDs(providerCounts) {
		census.Providers = append(census.Providers, providerCounts[providerID])
	}
	published, found := providerCounts[publishedProvider]
	if !found {
		return LegacyCacheCensus{}, fmt.Errorf("published provider namespace %q is absent", publishedProvider)
	}
	census.PublishedNamespace = LegacyPublishedNamespace{
		ProviderID: publishedProvider, Files: published.Files, Bytes: published.Bytes,
		ScopeStatus: legacyCacheScopeUpperBound, ExactRequestMap: false,
	}
	census.LegacyOnlyEntries = census.ResponseFiles
	census.ScientificInventoryDigest, err = protocol.Digest(scientificRecords)
	if err != nil {
		return LegacyCacheCensus{}, err
	}
	census.OperationalMetadataDigest, err = protocol.Digest(operationalRecords)
	if err != nil {
		return LegacyCacheCensus{}, err
	}
	census.Digest, err = legacyCacheCensusDigest(census)
	if err != nil {
		return LegacyCacheCensus{}, err
	}
	return census, census.Validate()
}

func (census LegacyCacheCensus) Validate() error {
	if census.SchemaVersion != LegacyCacheCensusSchemaVersion || census.InventoryAlgorithm != legacyCacheInventoryAlgorithm ||
		census.ResponseSchemaVersion != 1 || census.TotalFiles < 1 || census.TotalBytes < 1 ||
		census.ResponseFiles < 1 || census.ResponseBytes < 1 || census.CapabilityFiles < 0 || census.CapabilityBytes < 0 ||
		census.OperationalFiles < 0 || census.OperationalBytes < 0 || len(census.Providers) == 0 ||
		!validLegacyProviderID(census.PublishedNamespace.ProviderID) || census.PublishedNamespace.Files < 1 ||
		census.PublishedNamespace.Bytes < 1 || census.PublishedNamespace.ScopeStatus != legacyCacheScopeUpperBound ||
		census.PublishedNamespace.ExactRequestMap || !validLegacyDigest(census.ScientificInventoryDigest) ||
		!validLegacyDigest(census.OperationalMetadataDigest) || census.ExactAdmissibleEntries != 0 ||
		census.LegacyOnlyEntries != census.ResponseFiles || !slices.Equal(census.MissingIdentities, legacyCensusMissingIdentities) ||
		census.OperationalContentRead || census.SensitiveContentEmitted || !census.ReadOnly || census.ProviderCalls != 0 ||
		!validLegacyDigest(census.Digest) {
		return errors.New("legacy cache census identity or evidence boundary is invalid")
	}
	files := census.CapabilityFiles + census.OperationalFiles
	bytesTotal := census.CapabilityBytes + census.OperationalBytes
	previous := ""
	publishedFound := false
	for _, provider := range census.Providers {
		if !validLegacyProviderID(provider.ProviderID) || provider.ProviderID <= previous || provider.Files < 1 || provider.Bytes < 1 {
			return errors.New("legacy cache provider census is invalid or unsorted")
		}
		files += provider.Files
		bytesTotal += provider.Bytes
		if provider.ProviderID == census.PublishedNamespace.ProviderID {
			publishedFound = provider.Files == census.PublishedNamespace.Files && provider.Bytes == census.PublishedNamespace.Bytes
		}
		previous = provider.ProviderID
	}
	if !publishedFound || files != census.TotalFiles || bytesTotal != census.TotalBytes ||
		census.ResponseFiles != files-census.CapabilityFiles-census.OperationalFiles ||
		census.ResponseBytes != bytesTotal-census.CapabilityBytes-census.OperationalBytes {
		return errors.New("legacy cache census totals are inconsistent")
	}
	digest, err := legacyCacheCensusDigest(census)
	if err != nil || digest != census.Digest {
		return errors.New("legacy cache census digest is invalid")
	}
	return nil
}

func EncodeLegacyCacheCensus(census LegacyCacheCensus) ([]byte, error) {
	if err := census.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(census)
}

func DecodeLegacyCacheCensus(raw []byte) (LegacyCacheCensus, error) {
	raw = bytes.TrimSuffix(raw, []byte{'\n'})
	var census LegacyCacheCensus
	if err := protocol.DecodeStrict(raw, &census); err != nil {
		return LegacyCacheCensus{}, err
	}
	canonical, err := protocol.CanonicalMarshal(census)
	if err != nil || !bytes.Equal(canonical, raw) {
		return LegacyCacheCensus{}, errors.New("legacy cache census is not canonical JSON")
	}
	return census, census.Validate()
}

func readLegacyCensusFile(root *os.Root, relative string, walked fs.FileInfo) ([]byte, error) {
	file, err := root.Open(relative)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(walked, opened) {
		_ = file.Close()
		return nil, errors.Join(err, errors.New("legacy cache file changed before read"))
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maxEntryBytes+1))
	final, statErr := file.Stat()
	pathInfo, pathErr := root.Lstat(relative)
	closeErr := file.Close()
	if readErr != nil || statErr != nil || pathErr != nil || closeErr != nil || int64(len(raw)) != walked.Size() ||
		!pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, final) ||
		!os.SameFile(final, pathInfo) || opened.Size() != final.Size() || !opened.ModTime().Equal(final.ModTime()) {
		return nil, errors.Join(readErr, statErr, pathErr, closeErr, errors.New("legacy cache file changed during read"))
	}
	return raw, nil
}

func observeLegacyCensusFileMetadata(root *os.Root, relative string, walked fs.FileInfo) error {
	file, err := root.Open(relative)
	if err != nil {
		return err
	}
	opened, statErr := file.Stat()
	final, finalErr := file.Stat()
	pathInfo, pathErr := root.Lstat(relative)
	closeErr := file.Close()
	if statErr != nil || finalErr != nil || pathErr != nil || closeErr != nil ||
		!opened.Mode().IsRegular() || !final.Mode().IsRegular() || !pathInfo.Mode().IsRegular() ||
		pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(walked, opened) ||
		!os.SameFile(opened, final) || !os.SameFile(final, pathInfo) ||
		walked.Size() != opened.Size() || !walked.ModTime().Equal(opened.ModTime()) ||
		opened.Size() != final.Size() || !opened.ModTime().Equal(final.ModTime()) {
		return errors.Join(statErr, finalErr, pathErr, closeErr, errors.New("legacy operational file changed during metadata observation"))
	}
	return nil
}

func sortedLegacyProviderIDs(values map[string]LegacyProviderCensus) []string {
	result := make([]string, 0, len(values))
	for providerID := range values {
		result = append(result, providerID)
	}
	slices.Sort(result)
	return result
}

func legacyCacheCensusDigest(census LegacyCacheCensus) (string, error) {
	census.Digest = ""
	return protocol.Digest(census)
}

func validLegacyProviderID(value string) bool {
	if value == "" || len(value) > 128 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validLegacyDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
