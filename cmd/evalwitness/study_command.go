package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

type repeatedStringFlag []string

func (flagValues *repeatedStringFlag) String() string {
	return strings.Join(*flagValues, ",")
}

func (flagValues *repeatedStringFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value must not be empty")
	}
	*flagValues = append(*flagValues, value)
	return nil
}

func runStudy(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "study: usage: evalwitness study <validate|lock|transition|report|split|schema|inventory|verify-execution>")
		return 2
	}
	switch args[0] {
	case "validate":
		return runStudyValidate(args[1:])
	case "lock":
		return runStudyLock(args[1:])
	case "transition":
		return runStudyTransition(args[1:])
	case "report":
		return runStudyReport(args[1:])
	case "split":
		return runStudySplit(args[1:])
	case "schema":
		return runStudySchema(args[1:])
	case "inventory":
		return runStudyInventory(args[1:])
	case "identical-response-inventory":
		return runStudyIdenticalResponseInventory(args[1:])
	case "identical-response-redistribution-right":
		return runStudyIdenticalResponseRedistributionRight(args[1:])
	case "identical-response-protocol":
		return runStudyIdenticalResponseProtocol(args[1:])
	case "verify-execution":
		return runStudyVerifyExecution(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "study: unknown command %q\n", args[0])
		return 2
	}
}

func runStudyValidate(args []string) int {
	fs := flag.NewFlagSet("study validate", flag.ContinueOnError)
	path := fs.String("manifest", "", "manifest JSON (@file or - for stdin)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	manifest, closeDocument, err := readStudyManifest(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study validate:", err)
		return 2
	}
	defer closeDocument()
	digest, err := study.ManifestDigest(manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study validate:", err)
		return 2
	}
	return printStudyJSON(struct {
		SchemaVersion  string `json:"schema_version"`
		Valid          bool   `json:"valid"`
		ManifestDigest string `json:"manifest_digest"`
	}{SchemaVersion: study.ManifestSchemaVersion, Valid: true, ManifestDigest: digest})
}

func runStudyLock(args []string) int {
	fs := flag.NewFlagSet("study lock", flag.ContinueOnError)
	path := fs.String("manifest", "", "manifest JSON (@file or - for stdin)")
	actor := fs.String("actor", "", "person or system locking the study")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	manifest, closeDocument, err := readStudyManifest(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study lock:", err)
		return 2
	}
	defer closeDocument()
	record, err := study.Lock(manifest, *actor)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study lock:", err)
		return 2
	}
	return printStudyJSON(record)
}

func runStudyTransition(args []string) int {
	fs := flag.NewFlagSet("study transition", flag.ContinueOnError)
	path := fs.String("record", "", "current immutable study record (@file or - for stdin)")
	to := fs.String("to", "", "authorized/running/complete/failed/withdrawn")
	at := fs.String("at", "", "RFC3339 transition timestamp")
	actor := fs.String("actor", "", "person or system making the transition")
	reason := fs.String("reason", "", "prespecified transition reason")
	var attestations repeatedStringFlag
	fs.Var(&attestations, "attestation-digest", "current arm attestation SHA-256; repeat once per arm")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	record, closeDocument, err := readStudyRecord(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study transition:", err)
		return 2
	}
	defer closeDocument()
	transitionAt, err := time.Parse(time.RFC3339Nano, *at)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study transition: --at must be RFC3339:", err)
		return 2
	}
	next, err := study.Transition(record, study.State(*to), transitionAt, *actor, *reason, attestations)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study transition:", err)
		return 2
	}
	return printStudyJSON(next)
}

func runStudyReport(args []string) int {
	fs := flag.NewFlagSet("study report", flag.ContinueOnError)
	path := fs.String("record", "", "immutable study record (@file or - for stdin)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	record, closeDocument, err := readStudyRecord(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study report:", err)
		return 2
	}
	defer closeDocument()
	report, err := study.ProtocolReport(record)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study report:", err)
		return 2
	}
	return writeCommandOutput("study report", []byte(report))
}

func runStudySplit(args []string) int {
	fs := flag.NewFlagSet("study split", flag.ContinueOnError)
	path := fs.String("request", "", "split request with spec and groups (@file or - for stdin)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study split:", err)
		return 2
	}
	defer closeDocument()
	var request struct {
		Spec   study.SplitSpec    `json:"spec"`
		Groups []study.SplitGroup `json:"groups"`
	}
	if err := study.DecodeStrict(reader, &request); err != nil {
		fmt.Fprintln(os.Stderr, "study split:", err)
		return 2
	}
	manifest, err := study.GenerateSplit(request.Spec, request.Groups)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study split:", err)
		return 2
	}
	return printStudyJSON(manifest)
}

func runStudySchema(args []string) int {
	fs := flag.NewFlagSet("study schema", flag.ContinueOnError)
	document := fs.String("type", "manifest", "manifest/record/split")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	schema, err := study.Schema(*document)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study schema:", err)
		return 2
	}
	return printStudyJSON(schema)
}

func runStudyInventory(args []string) int {
	fs := flag.NewFlagSet("study inventory", flag.ContinueOnError)
	path := fs.String("inventory", "eval/governance/development-inventory.json", "development inventory JSON (@file)")
	root := fs.String("root", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study inventory:", err)
		return 2
	}
	defer closeDocument()
	var inventory study.DevelopmentInventory
	if err := study.DecodeStrict(reader, &inventory); err != nil {
		fmt.Fprintln(os.Stderr, "study inventory:", err)
		return 2
	}
	if err := study.VerifyDevelopmentInventory(*root, inventory); err != nil {
		fmt.Fprintln(os.Stderr, "study inventory:", err)
		return 2
	}
	return printStudyJSON(struct {
		Valid    bool `json:"valid"`
		Datasets int  `json:"datasets"`
	}{Valid: true, Datasets: len(inventory.Datasets)})
}

func runStudyIdenticalResponseInventory(args []string) int {
	fs := flag.NewFlagSet("study identical-response-inventory", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root containing the governed controlled-corruption release")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "study identical-response-inventory: positional arguments are forbidden")
		return 2
	}
	inventory, err := study.BuildIdenticalResponseEligibleInventory(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study identical-response-inventory:", err)
		return 1
	}
	return printStudyJSON(inventory)
}

func runStudyIdenticalResponseRedistributionRight(args []string) int {
	fs := flag.NewFlagSet("study identical-response-redistribution-right", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "study identical-response-redistribution-right: positional arguments are forbidden")
		return 2
	}
	record, err := study.BuildIdenticalResponseRedistributionRight()
	if err != nil {
		fmt.Fprintln(os.Stderr, "study identical-response-redistribution-right:", err)
		return 1
	}
	return printStudyJSON(record)
}

func runStudyIdenticalResponseProtocol(args []string) int {
	fs := flag.NewFlagSet("study identical-response-protocol", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root containing the bound TASK 070 artifacts")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "study identical-response-protocol: positional arguments are forbidden")
		return 2
	}
	protocol, err := study.BuildIdenticalResponseProtocol(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study identical-response-protocol:", err)
		return 1
	}
	return printStudyJSON(protocol)
}

func runStudyVerifyExecution(args []string) int {
	fs := flag.NewFlagSet("study verify-execution", flag.ContinueOnError)
	recordPath := fs.String("record", "", "authorized study record (@file)")
	bindingPath := fs.String("binding", "", "observed execution binding (@file)")
	root := fs.String("root", ".", "repository root for declared input verification")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	record, closeRecord, err := readStudyRecord(*recordPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study verify-execution:", err)
		return 2
	}
	defer closeRecord()
	reader, closeBinding, err := openStudyDocument(*bindingPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study verify-execution:", err)
		return 2
	}
	defer closeBinding()
	var binding study.ExecutionBinding
	if err := study.DecodeStrict(reader, &binding); err != nil {
		fmt.Fprintln(os.Stderr, "study verify-execution:", err)
		return 2
	}
	if err := study.VerifyExecutionBinding(record, binding); err != nil {
		fmt.Fprintln(os.Stderr, "study verify-execution:", err)
		return 2
	}
	if err := study.VerifyDeclaredInputs(record, *root); err != nil {
		fmt.Fprintln(os.Stderr, "study verify-execution:", err)
		return 2
	}
	return printStudyJSON(struct {
		Authorized     bool   `json:"authorized"`
		StudyID        string `json:"study_id"`
		ManifestDigest string `json:"manifest_digest"`
	}{Authorized: true, StudyID: record.Study.StudyID, ManifestDigest: record.Study.ManifestDigest})
}

func readStudyManifest(path string) (study.Manifest, func(), error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return study.Manifest{}, func() {}, err
	}
	manifest, err := study.DecodeManifest(reader)
	if err != nil {
		closeDocument()
		return study.Manifest{}, func() {}, err
	}
	return manifest, closeDocument, nil
}

func readStudyRecord(path string) (study.Record, func(), error) {
	reader, closeDocument, err := openStudyDocument(path)
	if err != nil {
		return study.Record{}, func() {}, err
	}
	record, err := study.DecodeRecord(reader)
	if err != nil {
		closeDocument()
		return study.Record{}, func() {}, err
	}
	return record, closeDocument, nil
}

func openStudyDocument(path string) (io.Reader, func(), error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, func() {}, fmt.Errorf("document path is required")
	}
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	path = strings.TrimPrefix(path, "@")
	file, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return file, func() { _ = file.Close() }, nil
}

func printStudyJSON(value any) int {
	encoded, err := study.EncodeIndented(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, "study: encode output:", err)
		return 1
	}
	return writeCommandOutput("study", encoded)
}
