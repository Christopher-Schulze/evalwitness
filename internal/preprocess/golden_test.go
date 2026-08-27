package preprocess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type goldenFixtureManifest struct {
	SchemaVersion string               `json:"schema_version"`
	Fixtures      []goldenFixtureEntry `json:"fixtures"`
}

type goldenFixtureEntry struct {
	File                      string       `json:"file"`
	SourceFormat              SourceFormat `json:"source_format"`
	SourceDigest              string       `json:"source_digest"`
	TrajectoryDigest          string       `json:"trajectory_digest"`
	SourceRecords             int          `json:"source_records"`
	CanonicalEvents           int          `json:"canonical_events"`
	ProviderUsageObservations int          `json:"provider_usage_observations"`
}

func TestGoldenFixturesMatchConservationManifest(t *testing.T) {
	directory := filepath.Join("testdata", "golden")
	manifestBytes, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest goldenFixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "evalwitness.golden-fixture-manifest.v1" || len(manifest.Fixtures) != 6 {
		t.Fatalf("unexpected golden manifest: %+v", manifest)
	}
	for _, expected := range manifest.Fixtures {
		t.Run(expected.File, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(directory, expected.File))
			if err != nil {
				t.Fatal(err)
			}
			assertPublicFixture(t, string(raw))
			trajectory, err := IngestString(string(raw), DefaultIngestOptions())
			if err != nil {
				t.Fatal(err)
			}
			if trajectory.SourceFormat != expected.SourceFormat || trajectory.SourceDigest != expected.SourceDigest || trajectory.Digest != expected.TrajectoryDigest {
				t.Fatalf("fixture identity drifted: format=%s source=%s trajectory=%s", trajectory.SourceFormat, trajectory.SourceDigest, trajectory.Digest)
			}
			if trajectory.Report.SourceRecords != expected.SourceRecords || len(trajectory.Events) != expected.CanonicalEvents {
				t.Fatalf("fixture conservation drifted: records=%d events=%d", trajectory.Report.SourceRecords, len(trajectory.Events))
			}
			if len(trajectory.Report.ProviderUsage) != expected.ProviderUsageObservations {
				t.Fatalf("provider usage observations=%d, want %d", len(trajectory.Report.ProviderUsage), expected.ProviderUsageObservations)
			}
		})
	}
}

func assertPublicFixture(t *testing.T, value string) {
	t.Helper()
	for _, forbidden := range []string{"/Users/", "BEGIN OPENSSH PRIVATE KEY", "sk-", "ghp_", "private chain", "chain of thought"} {
		if strings.Contains(value, forbidden) {
			t.Fatalf("fixture contains forbidden private material %q", forbidden)
		}
	}
}

func TestCanonicalTrajectoryJSONRoundTrip(t *testing.T) {
	source, err := IngestString(claudeCodeFixture, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Trajectory
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	if decoded.Digest != source.Digest || RenderTrajectory(decoded) != RenderTrajectory(source) {
		t.Fatal("canonical JSON round trip changed identity or rendering")
	}
}
