package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/calibration"
)

func runCalibration(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "calibration: missing subcommand (evaluate|seal|verify|apply|bind-049|bind-034)")
		return 2
	}
	switch args[0] {
	case "evaluate":
		return runCalibrationEvaluate(args[1:])
	case "seal":
		return runCalibrationSeal(args[1:])
	case "verify":
		return runCalibrationVerify(args[1:])
	case "apply":
		return runCalibrationApply(args[1:])
	case "bind-049":
		return runCalibrationBind049(args[1:])
	case "bind-034":
		return runCalibrationBind034(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "calibration: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runCalibrationEvaluate(args []string) int {
	flags := flag.NewFlagSet("calibration evaluate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	observationsPath := flags.String("observations", "", "held-out Observation JSON array")
	threshold := flags.Float64("threshold", 0, "selection threshold on predicted probability")
	targetRisk := flags.Float64("target-risk", 0, "maximum selective-risk upper bound")
	minCoverage := flags.Float64("min-coverage", 0, "minimum coverage")
	seed := flags.Uint64("seed", 1, "bootstrap seed")
	artifactPath := flags.String("artifact", "", "optional sealed model artifact JSON")
	route := flags.String("route", "", "required with --artifact")
	domain := flags.String("domain", "", "required with --artifact")
	inventoryPath := flags.String("inventory", "", "optional TASK 034 development inventory; rejects confirmatory use of those tasks")
	root := flags.String("root", ".", "repository root for --inventory artifact bytes")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *observationsPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "calibration evaluate: --observations is required; positional arguments are forbidden")
		return 2
	}
	if (*artifactPath != "") != (*route != "" && *domain != "") {
		fmt.Fprintln(os.Stderr, "calibration evaluate: --artifact requires --route and --domain")
		return 2
	}
	raw, err := readBoundedCommandFile(*observationsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration evaluate:", err)
		return 1
	}
	observations, err := calibration.DecodeObservations(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration evaluate:", err)
		return 1
	}
	if err := calibration.Guard034Development(*root, *inventoryPath, observations); err != nil {
		fmt.Fprintln(os.Stderr, "calibration evaluate:", err)
		return 1
	}
	var evaluation calibration.DeploymentEvaluation
	if *artifactPath != "" {
		artifactRaw, readErr := readBoundedCommandFile(*artifactPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "calibration evaluate:", readErr)
			return 1
		}
		artifact, decodeErr := calibration.DecodeModelArtifact(artifactRaw)
		if decodeErr != nil {
			fmt.Fprintln(os.Stderr, "calibration evaluate:", decodeErr)
			return 1
		}
		evaluation, err = calibration.EvaluateDeploymentScoped(observations, *threshold, *targetRisk, *minCoverage, *seed, artifact, *route, *domain)
	} else {
		evaluation, err = calibration.EvaluateDeployment(observations, *threshold, *targetRisk, *minCoverage, *seed)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration evaluate:", err)
		return 1
	}
	encoded, err := calibration.EncodeDeploymentEvaluation(evaluation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration evaluate:", err)
		return 1
	}
	if code := writeCommandOutput("calibration evaluate", encoded); code != 0 {
		return code
	}
	if evaluation.Applicability != nil && !evaluation.Applicability.Applicable {
		return 1
	}
	return 0
}

func runCalibrationSeal(args []string) int {
	flags := flag.NewFlagSet("calibration seal", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	artifactPath := flags.String("artifact", "", "model artifact draft JSON")
	calibratorPath := flags.String("calibrator", "", "calibrator JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *artifactPath == "" || *calibratorPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "calibration seal: --artifact and --calibrator are required; positional arguments are forbidden")
		return 2
	}
	artifact, calibratorRaw, err := readCalibrationArtifactPair(*artifactPath, *calibratorPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration seal:", err)
		return 1
	}
	sealed, err := calibration.SealModelArtifactBytes(artifact, calibratorRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration seal:", err)
		return 1
	}
	encoded, err := calibration.EncodeModelArtifact(sealed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration seal:", err)
		return 1
	}
	return writeCommandOutput("calibration seal", encoded)
}

func runCalibrationVerify(args []string) int {
	flags := flag.NewFlagSet("calibration verify", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	artifactPath := flags.String("artifact", "", "sealed model artifact JSON")
	calibratorPath := flags.String("calibrator", "", "calibrator JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *artifactPath == "" || *calibratorPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "calibration verify: --artifact and --calibrator are required; positional arguments are forbidden")
		return 2
	}
	artifact, calibratorRaw, err := readCalibrationArtifactPair(*artifactPath, *calibratorPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration verify:", err)
		return 1
	}
	if err := calibration.VerifyModelArtifactBytes(artifact, calibratorRaw); err != nil {
		fmt.Fprintln(os.Stderr, "calibration verify:", err)
		return 1
	}
	encoded, err := calibration.EncodeModelArtifact(artifact)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration verify:", err)
		return 1
	}
	return writeCommandOutput("calibration verify", encoded)
}

func readCalibrationArtifactPair(artifactPath, calibratorPath string) (calibration.ModelArtifact, []byte, error) {
	artifactRaw, err := readBoundedCommandFile(artifactPath)
	if err != nil {
		return calibration.ModelArtifact{}, nil, err
	}
	artifact, err := calibration.DecodeModelArtifact(artifactRaw)
	if err != nil {
		return calibration.ModelArtifact{}, nil, err
	}
	calibratorRaw, err := readBoundedCommandFile(calibratorPath)
	if err != nil {
		return calibration.ModelArtifact{}, nil, err
	}
	return artifact, calibratorRaw, nil
}

func runCalibrationApply(args []string) int {
	flags := flag.NewFlagSet("calibration apply", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	artifactPath := flags.String("artifact", "", "sealed model artifact JSON")
	calibratorPath := flags.String("calibrator", "", "optional calibrator JSON to verify before apply")
	route := flags.String("route", "", "decision route scope")
	domain := flags.String("domain", "", "decision domain scope")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *artifactPath == "" || *route == "" || *domain == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "calibration apply: --artifact, --route, and --domain are required; positional arguments are forbidden")
		return 2
	}
	artifactRaw, err := readBoundedCommandFile(*artifactPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration apply:", err)
		return 1
	}
	artifact, err := calibration.DecodeModelArtifact(artifactRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration apply:", err)
		return 1
	}
	if *calibratorPath != "" {
		calibratorRaw, readErr := readBoundedCommandFile(*calibratorPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "calibration apply:", readErr)
			return 1
		}
		if err := calibration.VerifyModelArtifactBytes(artifact, calibratorRaw); err != nil {
			fmt.Fprintln(os.Stderr, "calibration apply:", err)
			return 1
		}
	}
	decision := calibration.MatchApplicability(artifact, *route, *domain)
	code := writeCanonicalCommandOutput("calibration apply", decision)
	if !decision.Applicable {
		return 1
	}
	return code
}

func runCalibrationBind049(args []string) int {
	flags := flag.NewFlagSet("calibration bind-049", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	splitPath := flags.String("split", "", "frozen 049 split JSON")
	studyPath := flags.String("study", "", "049 study record JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *splitPath == "" || *studyPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "calibration bind-049: --split and --study are required; positional arguments are forbidden")
		return 2
	}
	report, err := calibration.Bind049ReportFromFiles(*splitPath, *studyPath, calibration.FeatureSchema{
		Version: "v1", Keys: []string{"conditional_diff"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration bind-049:", err)
		return 1
	}
	return writeCanonicalCommandOutput("calibration bind-049", report)
}

func runCalibrationBind034(args []string) int {
	flags := flag.NewFlagSet("calibration bind-034", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	inventoryPath := flags.String("inventory", "", "committed development-inventory JSON")
	root := flags.String("root", ".", "repository root containing the inventory artifacts")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *inventoryPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "calibration bind-034: --inventory is required; positional arguments are forbidden")
		return 2
	}
	report, err := calibration.Bind034Development(*root, *inventoryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "calibration bind-034:", err)
		return 1
	}
	return writeCanonicalCommandOutput("calibration bind-034", report)
}
