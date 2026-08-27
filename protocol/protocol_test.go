package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormativeCorpusPassesReferenceEvaluator(t *testing.T) {
	corpus := loadTestCorpus(t)
	run, err := RunReferenceCorpus(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 188 {
		t.Fatalf("normative cases = %d, want frozen protocol 1.2 corpus size 188", len(run.Results))
	}
	for _, result := range run.Results {
		if result.Outcome != OutcomePassed {
			t.Errorf("case %s outcome=%s reason=%s", result.CaseID, result.Outcome, result.Reason)
		}
	}
	if !validDigest(run.RunDigest) || !validDigest(run.CorpusDigest) || !validDigest(run.RequestCorpusDigest) || !validDigest(run.SchemaArtifactDigest) {
		t.Fatal("audit run did not bind all corpus and result digests")
	}
	if run.CorpusDigest != "b1526e257a27cf905d9a103eefdeb8a6dc56282f97558fe5710ea819d37c0ec3" ||
		run.RequestCorpusDigest != "198e4af31223975c1f2bbb76486a77486c5d82526b8ab8d286b1fb8a50398278" ||
		run.SchemaArtifactDigest != "7ec73192676d2c98dd2b9eee294ee8681d31c31c2750a2930eaae8a72e223873" {
		t.Fatal("frozen protocol 1.2 artifact identity changed without a version update")
	}
	if got := run.Matrix.Statuses[2]; got.Level != LevelLiveScoreEvidence || got.NotRun != 1 {
		t.Fatalf("live capability status = %+v", got)
	}
}

func TestRequiredFieldCorpusCoverageIsFrozen(t *testing.T) {
	corpus := loadTestCorpus(t)
	count := 0
	for _, vector := range corpus.Vectors {
		if strings.HasPrefix(vector.CaseID, "required.") {
			count++
		}
	}
	if count != 111 {
		t.Fatalf("required-field mutations = %d, want frozen protocol 1.2 count 111", count)
	}
}

func TestNormativeCorpusConsumesFrozenRequestBytes(t *testing.T) {
	raw := readRequestCorpus(t)
	corpus := loadTestCorpus(t)
	if corpus.RequestCorpusDigest != DigestBytes(raw) {
		t.Fatal("request corpus bytes were translated before protocol binding")
	}
	requestCases := 0
	expectedDigests := make(map[string]string)
	var frozen frozenRequestCorpus
	if err := json.Unmarshal(raw, &frozen); err != nil {
		t.Fatal(err)
	}
	for _, item := range frozen.Cases {
		expectedDigests["request."+item.Name] = item.Fingerprint
	}
	run, err := RunReferenceCorpus(corpus)
	if err != nil {
		t.Fatal(err)
	}
	for _, vector := range corpus.Vectors {
		if vector.Kind == CaseRequestFingerprint && strings.HasPrefix(vector.CaseID, "request.") {
			requestCases++
		}
	}
	if requestCases != 4 {
		t.Fatalf("request fingerprint cases = %d, want 4 frozen vectors", requestCases)
	}
	for _, result := range run.Results {
		expected, ok := expectedDigests[result.CaseID]
		if !ok {
			continue
		}
		if result.ObservedDigest != expected {
			t.Fatalf("case %s emitted digest %s, want %s", result.CaseID, result.ObservedDigest, expected)
		}
	}
}

func TestReferenceAdapterProcessLifecycle(t *testing.T) {
	if hasArgument(os.Args, "--evalwitness-reference-adapter-helper") {
		if err := ServeReferenceAdapter(os.Stdin, os.Stdout); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	corpus := loadTestCorpus(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run, err := RunAdapterCorpus(ctx, AdapterCommand{
		Path: os.Args[0], Arguments: []string{"-test.run=TestReferenceAdapterProcessLifecycle", "--", "--evalwitness-reference-adapter-helper"},
	}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range run.Results {
		if result.Outcome != OutcomePassed {
			t.Errorf("external case %s outcome=%s reason=%s", result.CaseID, result.Outcome, result.Reason)
		}
	}
}

func TestAdapterContextCancellationKillsProcess(t *testing.T) {
	if hasArgument(os.Args, "--evalwitness-hanging-adapter-helper") {
		time.Sleep(30 * time.Second)
		return
	}
	corpus := loadTestCorpus(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := RunAdapterCorpus(ctx, AdapterCommand{
		Path: os.Args[0], Arguments: []string{"-test.run=TestAdapterContextCancellationKillsProcess", "--", "--evalwitness-hanging-adapter-helper"},
	}, corpus)
	if err == nil {
		t.Fatal("hanging adapter survived host cancellation")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("adapter cancellation took %s", elapsed)
	}
}

func TestReferenceAdapterRejectsOutOfOrderLifecycle(t *testing.T) {
	var input bytes.Buffer
	helloPayload, err := CanonicalMarshal(Hello{SupportedVersions: SupportedVersions(), CanonicalPolicy: CanonicalPolicy})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMessage(&input, AdapterMessage{SchemaVersion: MessageSchema, ProtocolVersion: CurrentVersion, MessageType: MessageHello, MessageID: "host.hello", Payload: helloPayload}); err != nil {
		t.Fatal(err)
	}
	boundaryPayload, err := CanonicalMarshal(RunBoundary{RunID: "run.invalid-order", CaseCount: 0, CorpusDigest: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMessage(&input, AdapterMessage{SchemaVersion: MessageSchema, ProtocolVersion: CurrentVersion, MessageType: MessageBeginRun, MessageID: "host.begin", RunID: "run.invalid-order", Payload: boundaryPayload}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := ServeReferenceAdapter(&input, &output); err == nil {
		t.Fatal("adapter accepted begin_run before descriptor exchange")
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
	if len(lines) != 2 {
		t.Fatalf("adapter replies = %d, want hello_ack and error", len(lines))
	}
	var reply AdapterMessage
	if err := DecodeStrict(lines[1], &reply); err != nil {
		t.Fatal(err)
	}
	var protocolError ProtocolError
	if reply.MessageType != MessageError || DecodeStrict(reply.Payload, &protocolError) != nil || protocolError.Code != "protocol.state.invalid" {
		t.Fatalf("out-of-order reply = %+v payload=%s", reply, reply.Payload)
	}
}

func TestAdapterStdoutContaminationFailsRun(t *testing.T) {
	if hasArgument(os.Args, "--evalwitness-contaminated-adapter-helper") {
		if err := ServeReferenceAdapter(os.Stdin, os.Stdout); err != nil {
			os.Exit(2)
		}
		if _, err := fmt.Fprintln(os.Stdout, "diagnostic-on-stdout"); err != nil {
			t.Errorf("write contamination marker: %v", err)
		}
		return
	}
	corpus := loadTestCorpus(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := RunAdapterCorpus(ctx, AdapterCommand{
		Path: os.Args[0], Arguments: []string{"-test.run=TestAdapterStdoutContaminationFailsRun", "--", "--evalwitness-contaminated-adapter-helper"},
	}, corpus)
	if err == nil || !strings.Contains(err.Error(), "contamination") {
		t.Fatalf("stdout contamination error = %v", err)
	}
}

func TestCanonicalJSONRules(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		err  bool
	}{
		{name: "order", raw: `{"z":1,"a":"Grüße","nested":{"b":2,"a":1}}`, want: `{"a":"Grüße","nested":{"a":1,"b":2},"z":1}`},
		{name: "duplicate", raw: `{"a":1,"a":2}`, err: true},
		{name: "float", raw: `{"a":1.0}`, err: true},
		{name: "negative-zero", raw: `{"a":-0}`, err: true},
		{name: "unsafe-integer", raw: `{"a":9007199254740992}`, err: true},
		{name: "unpaired-high-surrogate", raw: `{"a":"\ud800"}`, err: true},
		{name: "unpaired-low-surrogate", raw: `{"a":"\udc00"}`, err: true},
		{name: "surrogate-pair", raw: `{"a":"\ud83d\ude00"}`, want: `{"a":"😀"}`},
		{name: "trailing", raw: `{} {}`, err: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CanonicalizeJSON([]byte(test.raw))
			if test.err {
				if err == nil {
					t.Fatalf("CanonicalizeJSON accepted %s", test.raw)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("canonical JSON = %s, want %s", got, test.want)
			}
		})
	}
}

func TestAbsentAndNullRemainDistinct(t *testing.T) {
	absent, err := CanonicalizeJSON([]byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	nullValue, err := CanonicalizeJSON([]byte(`{"a":1,"b":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(absent, nullValue) || DigestBytes(absent) == DigestBytes(nullValue) {
		t.Fatal("absent and explicit null collapsed to one identity")
	}
}

func TestExtensionPolicy(t *testing.T) {
	optional := Extension{Namespace: "com.example.optional", Schema: "com.example.optional.v1", Payload: json.RawMessage(`{"x":1}`)}
	if err := ValidateExtensions([]Extension{optional}, 1); err != nil {
		t.Fatal(err)
	}
	required := optional
	required.Required = true
	if err := ValidateExtensions([]Extension{required}, 1); err == nil {
		t.Fatal("unknown required extension was accepted")
	}
}

func TestProtocolMinorCompatibilityAndMajorRejection(t *testing.T) {
	if !supportedVersion(PreviousMinorVersion) || !supportedVersion(CurrentVersion) {
		t.Fatal("declared minor versions are not supported")
	}
	if supportedVersion("2.0.0") {
		t.Fatal("incompatible major version was accepted")
	}
	if PermittedClaim(LevelSyntax) == "" || strings.Contains(strings.ToLower(PermittedClaim(LevelSyntax)), "reliable") {
		t.Fatal("syntax claim is empty or overclaims reliability")
	}
}

func TestSchemasAreStrictCurrentArtifacts(t *testing.T) {
	names := []string{
		"adapter-message.schema.json", "audit-case.schema.json", "audit-finding.schema.json",
		"audit-invocation.schema.json", "audit-run.schema.json", "capability-matrix.schema.json",
		"decision-evidence.schema.json", "evaluator-descriptor.schema.json",
		"invocation-result.schema.json", "reliability-extension.schema.json",
		"score-evidence.schema.json", "shared.schema.json",
	}
	for _, name := range names {
		raw, err := ReadSchemaArtifact(name)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(raw) || !bytes.Contains(raw, []byte(SchemaDialect)) {
			t.Fatalf("schema %s is invalid or not draft 2020-12", name)
		}
		if name != "shared.schema.json" && !bytes.Contains(raw, []byte(`"additionalProperties": false`)) {
			t.Fatalf("schema %s does not close core fields", name)
		}
	}
}

func TestSchemaReferencesResolveToEmbeddedArtifacts(t *testing.T) {
	names := []string{
		"adapter-message.schema.json", "audit-case.schema.json", "audit-finding.schema.json",
		"audit-invocation.schema.json", "audit-run.schema.json", "capability-matrix.schema.json",
		"decision-evidence.schema.json", "evaluator-descriptor.schema.json",
		"invocation-result.schema.json", "reliability-extension.schema.json",
		"score-evidence.schema.json", "shared.schema.json",
	}
	artifacts := make(map[string]any, len(names))
	for _, name := range names {
		raw, err := ReadSchemaArtifact(name)
		if err != nil {
			t.Fatal(err)
		}
		var schema any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		artifacts[name] = schema
	}
	for name, schema := range artifacts {
		for _, reference := range schemaReferences(schema) {
			targetName, fragment, _ := strings.Cut(reference, "#")
			if targetName == "" {
				targetName = name
			}
			target, ok := artifacts[targetName]
			if !ok {
				t.Fatalf("schema %s references missing artifact %s", name, targetName)
			}
			if fragment != "" && !schemaFragmentExists(target, fragment) {
				t.Fatalf("schema %s reference %s has no target", name, reference)
			}
		}
	}
}

func TestJSONPointerEscapesAndInvalidSequences(t *testing.T) {
	want := []string{"a/b", "m~n"}
	got, err := jsonPointerTokens("/a~1b/m~0n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded pointer = %#v, want %#v", got, want)
	}
	for _, pointer := range []string{"/~", "/~2", "no-slash"} {
		if _, err := jsonPointerTokens(pointer); err == nil {
			t.Fatalf("invalid pointer %q was accepted", pointer)
		}
	}
}

func TestStrictExtensionDecoderRejectsMalformedTrailingData(t *testing.T) {
	var destination map[string]any
	for _, raw := range []string{`{"x":1}{"y":2}`, `{"x":1} trailing`} {
		if err := strictUnmarshal(json.RawMessage(raw), &destination); err == nil {
			t.Fatalf("strict extension decoder accepted %q", raw)
		}
	}
}

func TestCapabilityMatrixIsStable(t *testing.T) {
	results := []CaseResult{
		{CaseID: "a", Level: LevelSyntax, Outcome: OutcomePassed},
		{CaseID: "b", Level: LevelSyntax, Outcome: OutcomeFailed, Reason: "bad"},
	}
	first := BuildCapabilityMatrix(results)
	second := BuildCapabilityMatrix(append([]CaseResult(nil), results...))
	if !reflect.DeepEqual(first, second) || first.Statuses[0].Passed != 1 || first.Statuses[0].Failed != 1 {
		t.Fatalf("capability matrix is unstable: %+v", first)
	}
}

func FuzzCanonicalizeJSONNeverPanics(f *testing.F) {
	for _, seed := range []string{`{}`, `{"a":1}`, `{"a":1,"a":2}`, `[]`, `null`, `{"x":"🧪"}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		canonical, err := CanonicalizeJSON([]byte(raw))
		if err != nil {
			return
		}
		if len(canonical) > 2<<20 || !json.Valid(canonical) {
			t.Fatal("canonical output is invalid or exceeds bounded expansion")
		}
	})
}

func loadTestCorpus(t *testing.T) NormativeCorpus {
	t.Helper()
	corpus, err := LoadNormativeCorpus(readRequestCorpus(t))
	if err != nil {
		t.Fatal(err)
	}
	return corpus
}

func readRequestCorpus(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../internal/provider/testdata/request-fingerprint-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func hasArgument(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func schemaReferences(value any) []string {
	references := make([]string, 0)
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "$ref" {
					if reference, ok := child.(string); ok {
						references = append(references, reference)
					}
					continue
				}
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(value)
	return references
}

func schemaFragmentExists(schema any, fragment string) bool {
	if !strings.HasPrefix(fragment, "/") {
		return false
	}
	current := schema
	for _, token := range strings.Split(fragment[1:], "/") {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		current, ok = object[token]
		if !ok {
			return false
		}
	}
	return true
}
