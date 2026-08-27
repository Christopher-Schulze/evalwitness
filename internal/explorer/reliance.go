package explorer

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/reliance"
)

func AddEvidenceReliance(
	ctx context.Context,
	report Report,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claimledger.Ledger,
) (Report, error) {
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	if report.Reliance != nil || report.Capsule.CapsuleID != base.Manifest.CapsuleID ||
		report.Capsule.ManifestDigest != base.Manifest.ManifestDigest {
		return Report{}, errors.New("evidence explorer report is not the declared reliance base capsule")
	}
	projection, err := reliance.BuildRelianceExplorerProjection(ctx, base, child, ledger)
	if err != nil {
		return Report{}, err
	}
	cloned, err := cloneRelianceProjection(projection)
	if err != nil {
		return Report{}, err
	}
	report.Extensions, err = addRelianceExtension(report.Extensions, cloned.Source)
	if err != nil {
		return Report{}, err
	}
	report.Reliance, report.Digest = &cloned, ""
	return sealReport(report)
}

func cloneRelianceProjection(
	value reliance.RelianceExplorerProjection,
) (reliance.RelianceExplorerProjection, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return reliance.RelianceExplorerProjection{}, err
	}
	var result reliance.RelianceExplorerProjection
	if err := json.Unmarshal(raw, &result); err != nil {
		return reliance.RelianceExplorerProjection{}, err
	}
	return result, nil
}

func addRelianceExtension(
	views []ExtensionView,
	source reliance.RelianceExplorerArtifactRef,
) ([]ExtensionView, error) {
	result := cloneExtensionViews(views)
	found := 0
	for index := range result {
		if result[index].ExtensionID != "task-065-reliance" {
			continue
		}
		found++
		result[index].Components = []ExtensionComponentIdentity{{
			TypeID: source.SchemaVersion, ComponentID: source.ID,
		}}
		result[index].MissingTypes = []string{}
		result[index].Availability = AvailabilityAvailable
	}
	if found != 1 {
		return nil, errors.New("evidence explorer reliance extension contract is absent or duplicated")
	}
	return result, nil
}

func cloneExtensionViews(values []ExtensionView) []ExtensionView {
	result := slices.Clone(values)
	for index := range result {
		result[index].RequiredTypes = slices.Clone(values[index].RequiredTypes)
		result[index].Components = slices.Clone(values[index].Components)
		result[index].MissingTypes = slices.Clone(values[index].MissingTypes)
	}
	return result
}

func (report Report) validateReliance() error {
	extension, err := report.relianceExtension()
	if err != nil {
		return err
	}
	if report.Reliance == nil {
		if extension.Availability == AvailabilityAvailable {
			return errors.New("evidence explorer reliance extension is available without a verified view")
		}
		return nil
	}
	if err := report.Reliance.Validate(); err != nil {
		return err
	}
	if report.Reliance.BaseCapsuleID != report.Capsule.CapsuleID ||
		extension.Availability != AvailabilityAvailable || len(extension.Components) != 1 ||
		extension.Components[0].TypeID != report.Reliance.Source.SchemaVersion ||
		extension.Components[0].ComponentID != report.Reliance.Source.ID || len(extension.MissingTypes) != 0 {
		return errors.New("evidence explorer reliance view is detached from its capsule extension")
	}
	return nil
}

func (report Report) relianceExtension() (ExtensionView, error) {
	var result ExtensionView
	found := 0
	for _, extension := range report.Extensions {
		if extension.ExtensionID == "task-065-reliance" {
			result, found = extension, found+1
		}
	}
	if found != 1 {
		return ExtensionView{}, errors.New("evidence explorer reliance extension is absent or duplicated")
	}
	return result, nil
}
