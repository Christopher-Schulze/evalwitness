package mutation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCorpusSourceResolverLoadsEachIndexedPathOnce(t *testing.T) {
	tests := []struct {
		name      string
		locations []string
		want      []int
	}{
		{name: "canonical indexes", locations: []string{"corpus.json#/0", "corpus.json#/2"}, want: []int{0, 2}},
		{name: "equivalent index spelling", locations: []string{"corpus.json#/2", "corpus.json#/00"}, want: []int{2, 0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "corpus.json")
			if err := os.WriteFile(path, []byte(`[{"value":0},{"value":1},{"value":2}]`), 0o600); err != nil {
				t.Fatal(err)
			}
			sources := make([]CorpusSource, len(test.locations))
			for index, location := range test.locations {
				sources[index] = CorpusSource{ID: "source-" + location, SourceLocation: location}
			}
			locations, requested, err := planCorpusSourceReads(sources)
			if err != nil {
				t.Fatal(err)
			}
			resolvedRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			resolver := corpusSourceResolver{
				root: resolvedRoot, requestedItems: requested,
				indexedByPath: make(map[string]map[int][]byte), wholeByPath: make(map[string][]byte),
			}
			assertResolvedCorpusValue(t, resolver.read, locations[0], test.want[0])
			if err := os.WriteFile(path, []byte(`not-json`), 0o600); err != nil {
				t.Fatal(err)
			}
			assertResolvedCorpusValue(t, resolver.read, locations[1], test.want[1])
		})
	}
}

func assertResolvedCorpusValue(t *testing.T, read func(corpusSourceLocation) ([]byte, error), location corpusSourceLocation, want int) {
	t.Helper()
	raw, err := read(location)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value.Value != want {
		t.Fatalf("resolved value = %d, want %d", value.Value, want)
	}
}
