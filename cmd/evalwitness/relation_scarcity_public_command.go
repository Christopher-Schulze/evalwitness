package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
)

func runRelationRenderScarcityPublicBrief(args []string) int {
	flags := flag.NewFlagSet("relation render-scarcity-public-brief", flag.ContinueOnError)
	format := flags.String("format", "markdown", "output format: markdown or json")
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	primaryPath := flags.String("primary-sample", "", "governed v3 primary sample (@file)")
	sentinelPath := flags.String("scarcity-sentinel", "", "governed v3 scarcity sentinel (@file)")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		return relationReviewInputError("render-scarcity-public-brief", err)
	}
	primary, err := loadRelationPrimaryV3(*primaryPath, plan)
	if err != nil {
		return relationReviewInputError("render-scarcity-public-brief", err)
	}
	sentinel, err := loadRelationSentinelV3(*sentinelPath, plan, primary)
	if err != nil {
		return relationReviewInputError("render-scarcity-public-brief", err)
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		return relationReviewInputError("render-scarcity-public-brief", err)
	}
	evidence, err := relation.BuildScarcityPublicEvidence(plan, primary, sentinel, corpusPlan, audit, release)
	if err != nil {
		return relationReviewOperationError("render-scarcity-public-brief", err)
	}
	return writeScarcityPublicEvidence(*format, evidence)
}

func writeScarcityPublicEvidence(format string, evidence relation.ScarcityPublicEvidence) int {
	switch format {
	case "json":
		encoded, encodeErr := relation.EncodeIndented(evidence)
		if encodeErr != nil {
			return relationReviewOperationError("render-scarcity-public-brief", encodeErr)
		}
		return writeCommandOutput("relation render-scarcity-public-brief", encoded)
	case "markdown":
		rendered, renderErr := relation.RenderScarcityPublicBriefMarkdown(evidence)
		if renderErr != nil {
			return relationReviewOperationError("render-scarcity-public-brief", renderErr)
		}
		return writeCommandOutput("relation render-scarcity-public-brief", []byte(rendered))
	default:
		fmt.Fprintln(os.Stderr, "relation render-scarcity-public-brief: --format must be markdown or json")
		return 2
	}
}
