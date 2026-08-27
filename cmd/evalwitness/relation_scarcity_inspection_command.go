package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
)

func runRelationRenderScarcityInspection(args []string) int {
	flags := flag.NewFlagSet("relation render-scarcity-inspection", flag.ContinueOnError)
	rootPath := flags.String("root", ".", "repository root containing fetched trajectory sources")
	planPath := flags.String("plan", "", "governed v3 relation audit plan (@file)")
	primaryPath := flags.String("primary-sample", "", "governed v3 primary sample (@file)")
	sentinelPath := flags.String("scarcity-sentinel", "", "governed v3 scarcity sentinel (@file)")
	corpusPlanPath := flags.String("corpus-plan", "", "frozen v3 corpus development plan (@file)")
	auditPath := flags.String("corpus-audit", "", "frozen v3 corpus development audit (@file)")
	releasePath := flags.String("release", "", "frozen v3 controlled-corruption release (@file)")
	var materialPaths repeatedStringFlag
	flags.Var(&materialPaths, "material", "owner-only v3 sentinel case material (@file; repeat exactly once per case)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	plan, err := loadRelationPlanV3(*planPath)
	if err != nil {
		return relationReviewInputError("render-scarcity-inspection", err)
	}
	primary, err := loadRelationPrimaryV3(*primaryPath, plan)
	if err != nil {
		return relationReviewInputError("render-scarcity-inspection", err)
	}
	sentinel, err := loadRelationSentinelV3(*sentinelPath, plan, primary)
	if err != nil {
		return relationReviewInputError("render-scarcity-inspection", err)
	}
	materials := make([]relation.CaseMaterial, len(materialPaths))
	for index, path := range materialPaths {
		material, readErr := readPrivateRelationDocument(path, relation.DecodeCaseMaterial)
		if readErr != nil {
			return relationReviewInputError("render-scarcity-inspection", readErr)
		}
		materials[index] = material
	}
	corpusPlan, audit, release, err := loadRelationCorpusV3(*corpusPlanPath, *auditPath, *releasePath)
	if err != nil {
		return relationReviewInputError("render-scarcity-inspection", err)
	}
	if err := relation.VerifyScarcityInspectionReplay(*rootPath, plan, primary, sentinel, corpusPlan, audit, release, materials); err != nil {
		return relationReviewOperationError("render-scarcity-inspection", err)
	}
	rendered, err := relation.RenderScarcityInspectionMarkdown(plan, primary, sentinel, materials)
	if err != nil {
		return relationReviewOperationError("render-scarcity-inspection", err)
	}
	if _, err := fmt.Fprint(os.Stdout, rendered); err != nil {
		fmt.Fprintln(os.Stderr, "relation render-scarcity-inspection:", err)
		return 1
	}
	return 0
}
