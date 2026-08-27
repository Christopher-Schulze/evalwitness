package safety

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAIKeyPatternRequiresARealTokenBoundary(t *testing.T) {
	key := "sk-proj-EXAMPLEKEYNOTREAL0000000000"
	for _, text := range []string{key, "token=" + key, "Bearer " + key} {
		matches := FindSecretPatterns(text)
		if !containsSecretRule(matches, "openai_key") {
			t.Fatalf("OpenAI-style key was not detected in %q", text)
		}
		if strings.Contains(RedactSecretPatterns(text), key) {
			t.Fatalf("OpenAI-style key was not redacted in %q", text)
		}
	}

	artifactType := "evalwitness.task-049-controlled-corruption-development.v1"
	if containsSecretRule(FindSecretPatterns(artifactType), "openai_key") {
		t.Fatal("task schema identity was misclassified as an OpenAI key")
	}
	if RedactSecretPatterns(artifactType) != artifactType {
		t.Fatal("task schema identity was changed by secret redaction")
	}
}

func TestCredentialAssignmentIgnoresMinifiedBooleanLiterals(t *testing.T) {
	if containsSecretRule(FindSecretPatterns("var inputTypes={password:!0,search:!0}"), "credential_assignment") {
		t.Fatal("minified boolean property was misclassified as a credential assignment")
	}
	if !containsSecretRule(FindSecretPatterns("api_key=tiny"), "credential_assignment") {
		t.Fatal("four-character credential assignment was not detected")
	}
}

func containsSecretRule(matches []TextMatch, rule string) bool {
	for _, match := range matches {
		if match.Rule == rule {
			return true
		}
	}
	return false
}

func BenchmarkSecretPatternScanningTrackedV5(b *testing.B) {
	data, err := os.ReadFile("../../eval/governance/identical-response-capture-bai-flash-v5.jsonl")
	if err != nil {
		b.Skipf("tracked v5 capture unavailable: %v", err)
	}
	for _, pattern := range secretPatterns {
		b.Run(pattern.rule, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				_ = pattern.expression.FindAllIndex(data, -1)
			}
		})
	}
}
