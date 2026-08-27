package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func runProtocol(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "protocol: expected run, cases, schema, reference-adapter, or application-adapter")
		return 2
	}
	switch args[0] {
	case "run":
		return runProtocolCorpus(args[1:])
	case "cases":
		return runProtocolCases(args[1:])
	case "schema":
		return runProtocolSchema(args[1:])
	case "reference-adapter":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "protocol reference-adapter: no arguments accepted")
			return 2
		}
		if err := protocolkit.ServeReferenceAdapter(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "protocol reference-adapter:", err)
			return 1
		}
		return 0
	case "application-adapter":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "protocol application-adapter: no arguments accepted")
			return 2
		}
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "protocol application-adapter:", err)
			return 1
		}
		if cfg.ReplayFrom == "" && !cfg.Offline {
			fmt.Fprintln(os.Stderr, "protocol application-adapter: only an explicit offline or exact-replay route is permitted")
			return 2
		}
		service, err := verification.NewProductionService(cfg, version)
		if err != nil {
			fmt.Fprintln(os.Stderr, "protocol application-adapter:", err)
			return 1
		}
		if err := protocolkit.ServeAdapter(os.Stdin, os.Stdout, applicationProtocolEvaluator{service: service}); err != nil {
			fmt.Fprintln(os.Stderr, "protocol application-adapter:", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "protocol: unknown operation %q\n", args[0])
		return 2
	}
}

func runProtocolCorpus(args []string) int {
	fs := flag.NewFlagSet("protocol run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	output := fs.String("output", "json", "json/text")
	requestVectors := fs.String("request-vectors", "", "optional frozen TASK 043 request-vector corpus")
	adapter := fs.String("adapter", "", "operator-selected adapter executable; empty uses in-process reference")
	adapterCWD := fs.String("adapter-cwd", "", "operator-selected adapter working directory")
	timeout := fs.Duration("timeout", 60*time.Second, "external adapter process deadline")
	var adapterArguments stringSlice
	fs.Var(&adapterArguments, "adapter-arg", "literal adapter argument; repeat as needed")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "protocol run:", err)
		return 2
	}
	if fs.NArg() != 0 || *timeout <= 0 || *timeout > 10*time.Minute {
		fmt.Fprintln(os.Stderr, "protocol run: invalid positional arguments or timeout")
		return 2
	}
	requestRaw, err := loadProtocolRequestVectors(*requestVectors)
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocol run:", err)
		return 1
	}
	corpus, err := protocolkit.LoadNormativeCorpus(requestRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocol run:", err)
		return 1
	}
	var run protocolkit.AuditRun
	if *adapter == "" {
		run, err = protocolkit.RunReferenceCorpus(corpus)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		run, err = protocolkit.RunAdapterCorpus(ctx, protocolkit.AdapterCommand{
			Path: *adapter, Arguments: append([]string(nil), adapterArguments...), WorkingDirectory: *adapterCWD,
		}, corpus)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocol run:", err)
		return 1
	}
	if err := printProtocolRun(run, *output); err != nil {
		fmt.Fprintln(os.Stderr, "protocol run:", err)
		return 2
	}
	for _, result := range run.Results {
		if result.Outcome == protocolkit.OutcomeFailed {
			return 1
		}
	}
	return 0
}

func runProtocolCases(args []string) int {
	fs := flag.NewFlagSet("protocol cases", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	requestVectors := fs.String("request-vectors", "", "optional frozen TASK 043 request-vector corpus")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "protocol cases: invalid arguments")
		return 2
	}
	raw, err := loadProtocolRequestVectors(*requestVectors)
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocol cases:", err)
		return 1
	}
	corpus, err := protocolkit.LoadNormativeCorpus(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocol cases:", err)
		return 1
	}
	output := struct {
		SchemaVersion        string                   `json:"schema_version"`
		CorpusDigest         string                   `json:"corpus_digest"`
		RequestCorpusDigest  string                   `json:"request_corpus_digest"`
		SchemaArtifactDigest string                   `json:"schema_artifact_digest"`
		Cases                []protocolkit.CaseVector `json:"cases"`
	}{
		SchemaVersion: protocolkit.VectorCorpusSchema, CorpusDigest: corpus.Digest,
		RequestCorpusDigest: corpus.RequestCorpusDigest, SchemaArtifactDigest: corpus.SchemaArtifactDigest,
		Cases: corpus.Vectors,
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocol cases:", err)
		return 1
	}
	fmt.Println(string(encoded))
	return 0
}

func runProtocolSchema(args []string) int {
	fs := flag.NewFlagSet("protocol schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "audit-case.schema.json", "embedded schema filename")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "protocol schema: invalid arguments")
		return 2
	}
	raw, err := protocolkit.ReadSchemaArtifact(*name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocol schema:", err)
		return 1
	}
	fmt.Println(string(raw))
	return 0
}

func loadProtocolRequestVectors(path string) ([]byte, error) {
	if path == "" {
		return provider.RequestFingerprintVectorCorpus(), nil
	}
	limits, err := safety.DefaultUntrustedInputLimits(safety.InputProtocolAdapter)
	if err != nil {
		return nil, err
	}
	raw, err := safety.ReadAndValidateUntrustedJSON(path, limits)
	if err != nil {
		return nil, fmt.Errorf("read request vectors: %w", err)
	}
	return raw, nil
}

func printProtocolRun(run protocolkit.AuditRun, output string) error {
	switch output {
	case "json":
		encoded, err := json.MarshalIndent(run, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
	case "text":
		fmt.Printf("protocol=%s evaluator=%s cases=%d corpus=%s request_corpus=%s schemas=%s run=%s offline=%t\n",
			run.ProtocolVersion, run.Evaluator.EvaluatorID, len(run.Results), run.CorpusDigest,
			run.RequestCorpusDigest, run.SchemaArtifactDigest, run.RunDigest, run.Offline)
		for _, status := range run.Matrix.Statuses {
			fmt.Printf("  %s passed=%d failed=%d skipped=%d unsupported=%d not_run=%d\n",
				status.Level, status.Passed, status.Failed, status.Skipped, status.Unsupported, status.NotRun)
			for _, reason := range status.Reasons {
				fmt.Printf("    %s\n", reason)
			}
		}
	default:
		return fmt.Errorf("--output must be json or text")
	}
	return nil
}
