package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

type artifact struct {
	Usage   artifactUsage     `json:"usage"`
	Details []artifactDetails `json:"details"`
}

type artifactUsage struct {
	ExtractionMode   string `json:"extraction_mode"`
	UnextractedScore int    `json:"unextracted_scores"`
}

type artifactDetails struct {
	InstanceID     string             `json:"instance_id"`
	TaskName       string             `json:"task_name"`
	Rewards        []int              `json:"rewards"`
	SelectedIndex  *int               `json:"selected_index"`
	SelectedReward *int               `json:"selected_reward"`
	Selection      *artifactSelection `json:"selection"`
}

type artifactSelection struct {
	PairDecisions []artifactPairDecision `json:"pair_decisions"`
}

type artifactPairDecision struct {
	MeanDifference float64 `json:"mean_difference"`
}

type loadedArtifact struct {
	Path string
	Mode string
	Rows map[string]artifactDetails
}

type options struct {
	alpha      float64
	familySize int
	question   stats.InferenceQuestion
	margin     float64
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "paired-analysis:", err)
		os.Exit(2)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("paired-analysis", flag.ContinueOnError)
	alpha := fs.Float64("alpha", 0.05, "nominal type-I error rate")
	familySize := fs.Int("family-size", 1, "Bonferroni primary-family size")
	question := fs.String("question", string(stats.QuestionSuperiority), "superiority/non_inferiority/equivalence")
	margin := fs.Float64("margin", 0, "non-inferiority or equivalence margin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("usage: run-paired-analysis.sh [design flags] <details.json> [<details.json> ...]")
	}
	if !(*alpha > 0 && *alpha < 0.5) {
		return errors.New("alpha must be between zero and one half")
	}
	if *familySize < 1 {
		return errors.New("family-size must be positive")
	}
	opts := options{
		alpha: *alpha, familySize: *familySize,
		question: stats.InferenceQuestion(*question), margin: *margin,
	}
	loaded := make([]loadedArtifact, 0, fs.NArg())
	for _, path := range fs.Args() {
		item, err := loadArtifact(path)
		if err != nil {
			return err
		}
		loaded = append(loaded, item)
	}
	if len(loaded) == 2 && overlap(loaded[0].Rows, loaded[1].Rows) {
		return reportPaired(loaded[0], loaded[1], opts)
	}
	return reportAgainstRandom(loaded)
}

func loadArtifact(path string) (loadedArtifact, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return loadedArtifact{}, fmt.Errorf("read %s: %w", path, err)
	}
	var document artifact
	if err := json.Unmarshal(raw, &document); err != nil {
		return loadedArtifact{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(document.Details) == 0 {
		return loadedArtifact{}, fmt.Errorf("%s: no details array; rerun the eval with --details", path)
	}
	if document.Usage.UnextractedScore != 0 {
		return loadedArtifact{}, fmt.Errorf("%s: %d unextracted scores; artifact is not publishable", path, document.Usage.UnextractedScore)
	}
	rows := make(map[string]artifactDetails, len(document.Details))
	for _, row := range document.Details {
		key := row.InstanceID
		if key == "" {
			key = row.TaskName
		}
		if key == "" {
			return loadedArtifact{}, fmt.Errorf("%s: detail row without instance_id or task_name", path)
		}
		if _, exists := rows[key]; exists {
			return loadedArtifact{}, fmt.Errorf("%s: duplicate task %q", path, key)
		}
		if err := validateArtifactDetail(row); err != nil {
			return loadedArtifact{}, fmt.Errorf("%s: task %q: %w", path, key, err)
		}
		rows[key] = row
	}
	return loadedArtifact{Path: path, Mode: document.Usage.ExtractionMode, Rows: rows}, nil
}

func validateArtifactDetail(row artifactDetails) error {
	for _, reward := range row.Rewards {
		if reward != 0 && reward != 1 {
			return fmt.Errorf("non-binary reward %d", reward)
		}
	}
	if (row.SelectedIndex == nil) != (row.SelectedReward == nil) {
		return errors.New("selected_index and selected_reward must either both be present or both be absent")
	}
	if row.SelectedIndex == nil {
		return nil
	}
	if *row.SelectedIndex < 0 || *row.SelectedIndex >= len(row.Rewards) {
		return fmt.Errorf("selected_index %d is outside %d rewards", *row.SelectedIndex, len(row.Rewards))
	}
	if *row.SelectedReward != row.Rewards[*row.SelectedIndex] {
		return fmt.Errorf("selected_reward %d does not match rewards[%d]=%d", *row.SelectedReward, *row.SelectedIndex, row.Rewards[*row.SelectedIndex])
	}
	return nil
}

func reportPaired(a, b loadedArtifact, opts options) error {
	shared := sharedKeys(a.Rows, b.Rows)
	both, aOnly, bOnly, neither := 0, 0, 0, 0
	type flip struct {
		key    string
		aIndex *int
		bIndex *int
		winner string
	}
	flips := make([]flip, 0)
	for _, key := range shared {
		aRow, bRow := a.Rows[key], b.Rows[key]
		if !slices.Equal(aRow.Rewards, bRow.Rewards) {
			return fmt.Errorf("paired task %q has different reward vectors across arms", key)
		}
		if !stats.DecidableBinary(aRow.Rewards) || !stats.DecidableBinary(bRow.Rewards) || aRow.SelectedReward == nil || bRow.SelectedReward == nil {
			continue
		}
		aCorrect := *aRow.SelectedReward != 0
		bCorrect := *bRow.SelectedReward != 0
		switch {
		case aCorrect && bCorrect:
			both++
		case aCorrect:
			aOnly++
			flips = append(flips, flip{key: key, aIndex: aRow.SelectedIndex, bIndex: bRow.SelectedIndex, winner: "A"})
		case bCorrect:
			bOnly++
			flips = append(flips, flip{key: key, aIndex: aRow.SelectedIndex, bIndex: bRow.SelectedIndex, winner: "B"})
		default:
			neither++
		}
	}
	total := both + aOnly + bOnly + neither
	if total == 0 {
		return errors.New("no paired decidable tasks with selections")
	}
	discordant := aOnly + bOnly
	pValue := stats.McNemarExact(aOnly, bOnly)
	adjustedAlpha := opts.alpha / float64(opts.familySize)
	confidence, err := stats.PairedInferenceConfidence(opts.question, adjustedAlpha)
	if err != nil {
		return err
	}
	interval, err := stats.PairedBinaryScoreInterval(both, aOnly, bOnly, neither, confidence)
	if err != nil {
		return err
	}
	inference, err := stats.EvaluatePairedQuestion(opts.question, opts.margin, adjustedAlpha, interval, pValue)
	if err != nil {
		return err
	}
	region, err := stats.ExactMcNemarRejectionRegion(discordant, adjustedAlpha)
	if err != nil {
		return err
	}
	labelA := "A (" + armLabel(a, b) + ")"
	labelB := "B (" + armLabel(b, a) + ")"
	fmt.Printf("arm A: %s  mode=%s\n", a.Path, a.Mode)
	fmt.Printf("arm B: %s  mode=%s\n\n", b.Path, b.Mode)
	reportTies([]loadedArtifact{a, b})
	fmt.Printf("tasks shared: %d    decidable (paired): %d\n\n", len(shared), total)
	fmt.Printf("  both correct        %4d\n", both)
	fmt.Printf("  both wrong          %4d\n", neither)
	fmt.Printf("  %-18s %4d\n", labelA+" only", aOnly)
	fmt.Printf("  %-18s %4d\n", labelB+" only", bOnly)
	fmt.Printf("\n  discordant pairs    %4d\n", discordant)
	fmt.Printf("  paired effect A-B   %+.4f tasks  %.1f%% CI [%+.4f, %+.4f] (%s)\n",
		interval.Estimate, interval.Confidence*100, interval.Lower, interval.Upper, interval.Method)
	fmt.Printf("  McNemar exact, two-sided: p = %.4f  nominal alpha=%.4f  family-adjusted alpha=%.4f (Bonferroni m=%d)\n",
		pValue, opts.alpha, adjustedAlpha, opts.familySize)
	if region.UpperMin == nil {
		fmt.Println("  design resolution: no split can reject at the adjusted alpha for this discordant count")
	} else {
		fmt.Printf("  design resolution: smallest significant split %d:%d at adjusted alpha\n", *region.UpperMin, discordant-*region.UpperMin)
	}
	fmt.Printf("  typed inference: %s margin=%.4f -> %s\n", inference.Question, inference.Margin, inference.Conclusion)
	if flips != nil {
		fmt.Println("\ndisagreements (task, A picked, B picked, winner):")
		for _, item := range flips {
			fmt.Printf("  %-40s %4s %4s   %s\n", item.key, optionalInt(item.aIndex), optionalInt(item.bIndex), item.winner)
		}
	}
	return nil
}

func reportAgainstRandom(artifacts []loadedArtifact) error {
	mode := artifacts[0].Mode
	probabilities := make([]float64, 0)
	hits, skipped := 0, 0
	for _, item := range artifacts {
		if item.Mode != mode {
			return fmt.Errorf("pooling needs one extraction mode; got %q and %q", mode, item.Mode)
		}
		for _, key := range sortedKeys(item.Rows) {
			row := item.Rows[key]
			if !stats.DecidableBinary(row.Rewards) {
				continue
			}
			if row.SelectedReward == nil {
				skipped++
				continue
			}
			passes := 0
			for _, reward := range row.Rewards {
				passes += reward
			}
			probabilities = append(probabilities, float64(passes)/float64(len(row.Rewards)))
			if *row.SelectedReward != 0 {
				hits++
			}
		}
	}
	if len(probabilities) == 0 {
		return errors.New("no decidable tasks in the given artifacts")
	}
	pValue, err := stats.PoissonBinomialUpperTail(probabilities, hits)
	if err != nil {
		return err
	}
	expected := 0.0
	for _, probability := range probabilities {
		expected += probability
	}
	for _, item := range artifacts {
		fmt.Printf("arm: %s\n", item.Path)
	}
	fmt.Printf("mode: %s   (%d artifact(s) pooled)\n\n", mode, len(artifacts))
	reportTies(artifacts)
	fmt.Printf("decidable tasks:        %d\n", len(probabilities))
	if skipped > 0 {
		fmt.Printf("skipped (no selection): %d\n", skipped)
	}
	fmt.Printf("  solved by this arm    %6d\n", hits)
	fmt.Printf("  expected by random    %8.1f\n", expected)
	fmt.Printf("  effect vs random      %+8.4f tasks/task\n", (float64(hits)-expected)/float64(len(probabilities)))
	fmt.Printf("  Poisson-binomial exact, one-sided: p = %.2e  informative n=%d\n", pValue, len(probabilities))
	return nil
}

func reportTies(artifacts []loadedArtifact) {
	printed := false
	for _, item := range artifacts {
		ties, total := 0, 0
		for _, row := range item.Rows {
			if row.Selection == nil {
				continue
			}
			for _, decision := range row.Selection.PairDecisions {
				total++
				if decision.MeanDifference == 0 {
					ties++
				}
			}
		}
		if total == 0 {
			continue
		}
		if !printed {
			fmt.Println("tie rate (exact draws, resolved by fallback not evidence):")
			printed = true
		}
		fmt.Printf("  %-44s %-9s %4d/%-4d pairs tied  (%5.1f%%)\n", filepath.Base(item.Path), item.Mode, ties, total, 100*float64(ties)/float64(total))
	}
	if printed {
		fmt.Println()
	}
}

func overlap(a, b map[string]artifactDetails) bool {
	for key := range a {
		if _, found := b[key]; found {
			return true
		}
	}
	return false
}

func sharedKeys(a, b map[string]artifactDetails) []string {
	keys := make([]string, 0)
	for key := range a {
		if _, found := b[key]; found {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys(rows map[string]artifactDetails) []string {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func armLabel(item, other loadedArtifact) string {
	if item.Mode != other.Mode {
		return item.Mode
	}
	stem := strings.TrimSuffix(filepath.Base(item.Path), "-details.json")
	otherStem := strings.TrimSuffix(filepath.Base(other.Path), "-details.json")
	prefix := 0
	for prefix < len(stem) && prefix < len(otherStem) && stem[prefix] == otherStem[prefix] {
		prefix++
	}
	label := strings.TrimLeft(stem[prefix:], "-")
	if label == "" {
		return stem
	}
	return label
}

func optionalInt(value *int) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}
