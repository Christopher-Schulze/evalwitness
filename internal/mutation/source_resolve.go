package mutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type corpusSourceLocation struct {
	pathPart string
	item     int
	indexed  bool
}

type corpusSourceResolver struct {
	root           string
	requestedItems map[string]map[int]struct{}
	indexedByPath  map[string]map[int][]byte
	wholeByPath    map[string][]byte
}

func ResolveCorpusSources(root string, sources []CorpusSource) ([]SourceCandidate, error) {
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve corpus root: %w", err)
	}
	locations, requestedItems, err := planCorpusSourceReads(sources)
	if err != nil {
		return nil, err
	}
	resolver := corpusSourceResolver{
		root: resolvedRoot, requestedItems: requestedItems,
		indexedByPath: make(map[string]map[int][]byte, len(requestedItems)),
		wholeByPath:   make(map[string][]byte),
	}
	result := make([]SourceCandidate, 0, len(sources))
	for index, source := range sources {
		raw, readErr := resolver.read(locations[index])
		if readErr != nil {
			return nil, fmt.Errorf("resolve corpus source %q: %w", source.ID, readErr)
		}
		candidate, candidateErr := resolveCorpusCandidate(raw, source)
		if candidateErr != nil {
			return nil, candidateErr
		}
		result = append(result, candidate)
	}
	return result, nil
}

func planCorpusSourceReads(sources []CorpusSource) ([]corpusSourceLocation, map[string]map[int]struct{}, error) {
	locations := make([]corpusSourceLocation, len(sources))
	requestedItems := make(map[string]map[int]struct{})
	seen := make(map[string]struct{}, len(sources))
	for index, source := range sources {
		if _, duplicate := seen[source.ID]; duplicate {
			return nil, nil, fmt.Errorf("resolve corpus sources repeats %q", source.ID)
		}
		seen[source.ID] = struct{}{}
		pathPart, item, indexed, splitErr := splitCorpusSourceLocation(source.SourceLocation)
		if splitErr != nil {
			return nil, nil, fmt.Errorf("resolve corpus source %q: %w", source.ID, splitErr)
		}
		locations[index] = corpusSourceLocation{pathPart: pathPart, item: item, indexed: indexed}
		if indexed {
			if requestedItems[pathPart] == nil {
				requestedItems[pathPart] = make(map[int]struct{})
			}
			requestedItems[pathPart][item] = struct{}{}
		}
	}
	return locations, requestedItems, nil
}

func (r *corpusSourceResolver) read(location corpusSourceLocation) ([]byte, error) {
	if location.indexed {
		selected, loaded := r.indexedByPath[location.pathPart]
		if !loaded {
			var err error
			selected, err = readIndexedCorpusSource(r.root, location.pathPart, r.requestedItems[location.pathPart])
			if err != nil {
				return nil, err
			}
			r.indexedByPath[location.pathPart] = selected
		}
		raw, exists := selected[location.item]
		if !exists {
			return nil, fmt.Errorf("indexed source item %d is absent", location.item)
		}
		return raw, nil
	}
	if raw, loaded := r.wholeByPath[location.pathPart]; loaded {
		return raw, nil
	}
	raw, err := readWholeCorpusSource(r.root, location.pathPart)
	if err != nil {
		return nil, err
	}
	r.wholeByPath[location.pathPart] = raw
	return raw, nil
}

func resolveCorpusCandidate(raw []byte, source CorpusSource) (SourceCandidate, error) {
	reward, err := strconv.ParseFloat(source.Outcome.Value, 64)
	if err != nil {
		return SourceCandidate{}, fmt.Errorf("resolve corpus source %q reward: %w", source.ID, err)
	}
	entry := sourceCatalogEntry{
		Family: source.SourceFamily, Revision: source.SourceRevision, SPDX: source.License.SPDX,
		SourceURL: source.License.SourceURL, Redistribution: source.License.Redistribution, Attribution: source.License.Attribution,
	}
	candidate, err := newSourceCandidate(raw, source.TaskID, source.RepositoryID, source.ID, source.SourceLocation, reward, entry)
	if err != nil {
		return SourceCandidate{}, fmt.Errorf("ingest corpus source %q: %w", source.ID, err)
	}
	if err := validateReplayCandidate(source, candidate); err != nil {
		return SourceCandidate{}, fmt.Errorf("resolve corpus source %q: %w", source.ID, err)
	}
	return candidate, nil
}

func readWholeCorpusSource(root, pathPart string) ([]byte, error) {
	resolvedPath, err := resolveCorpusSourcePath(root, pathPart)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	if len(raw) > maximumCorpusSourceBytes {
		return nil, errors.New("source exceeds 64 MiB")
	}
	return raw, nil
}

func readIndexedCorpusSource(root, pathPart string, requested map[int]struct{}) (selected map[int][]byte, returnErr error) {
	if len(requested) == 0 {
		return nil, errors.New("indexed source requires at least one item")
	}
	resolvedPath, err := resolveCorpusSourcePath(root, pathPart)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = closeErr
		}
	}()
	decoder := json.NewDecoder(file)
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		return nil, errors.New("indexed source is not a JSON array")
	}
	selected = make(map[int][]byte, len(requested))
	for index := 0; decoder.More(); index++ {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		if _, needed := requested[index]; !needed {
			continue
		}
		if len(raw) > maximumCorpusSourceBytes {
			return nil, errors.New("indexed source item exceeds 64 MiB")
		}
		selected[index] = append([]byte(nil), raw...)
		if len(selected) == len(requested) {
			return selected, nil
		}
	}
	return selected, nil
}

func resolveCorpusSourcePath(root, pathPart string) (string, error) {
	if unsafePublicLocation(pathPart) {
		return "", errors.New("source location is not repository-relative")
	}
	path := filepath.Join(root, filepath.FromSlash(pathPart))
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("source path resolves outside the corpus root")
	}
	return resolvedPath, nil
}

func splitCorpusSourceLocation(location string) (string, int, bool, error) {
	separator := strings.LastIndex(location, "#/")
	if separator < 0 {
		return location, 0, false, nil
	}
	pathPart := location[:separator]
	indexText := location[separator+2:]
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 || pathPart == "" {
		return "", 0, false, errors.New("source array location has an invalid item index")
	}
	return pathPart, index, true, nil
}
