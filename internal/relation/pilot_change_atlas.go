package relation

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const PilotChangeAtlasPolicy = "evalwitness.relation-pilot-change-atlas.v1"

const pilotChangeAtlasContextLines = 3

type pilotLineChange struct {
	CommonPrefixLines  int
	CommonSuffixLines  int
	OriginalChanged    []string
	TransformedChanged []string
	PrefixContext      []string
	SuffixContext      []string
}

func RenderPilotChangeAtlasMarkdown(readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) (string, error) {
	return renderPilotChangeAtlasMarkdown("", readiness, bundle, mappings)
}

func RenderPilotChangeAtlasWithReceiptMarkdown(receipt PilotChangeReceipt, readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) (string, error) {
	if err := VerifyPilotChangeReceipt(receipt, readiness, bundle, mappings); err != nil {
		return "", err
	}
	return renderPilotChangeAtlasMarkdown(receipt.Digest, readiness, bundle, mappings)
}

func renderPilotChangeAtlasMarkdown(receiptDigest string, readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) (string, error) {
	if err := VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return "", err
	}
	mappingByPacket := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByPacket[mapping.PacketID] = mapping
	}

	var builder strings.Builder
	fmt.Fprintln(&builder, "# EvalWitness Owner-Only Relation Pilot Change Atlas")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "- Atlas policy: `%s`\n", PilotChangeAtlasPolicy)
	fmt.Fprintf(&builder, "- Readiness digest: `%s`\n", readiness.Digest)
	fmt.Fprintf(&builder, "- Bundle digest: `%s`\n", bundle.Digest)
	fmt.Fprintf(&builder, "- Mapping commitment: `%s`\n", readiness.MappingCommitmentDigest)
	if receiptDigest != "" {
		fmt.Fprintf(&builder, "- Change receipt digest: `%s`\n", receiptDigest)
	}
	fmt.Fprintf(&builder, "- Packets: %d\n", readiness.Packets)
	fmt.Fprintf(&builder, "- External action: `%s`\n\n", readiness.ExternalActionStatus)
	fmt.Fprintln(&builder, "> OWNER-ONLY RESTRICTED NAVIGATION AID. This atlas shows every changed rendered line and hidden logical mapping. It does not replace the complete workbook, make a semantic decision, establish construct validity, or authorize any external action.")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "Each trajectory-pair section binds the complete original and transformed content digests, counts every unchanged prefix and suffix line, and prints every non-common line with bounded context. Candidate reversal is checked by exact content identity in reverse source-candidate order.")

	for index, packet := range bundle.Packets {
		mapping := mappingByPacket[packet.PacketID]
		fmt.Fprintf(&builder, "\n## Packet %d of %d: `%s`\n\n", index+1, len(bundle.Packets), mapping.Family)
		fmt.Fprintf(&builder, "- Packet ID: `%s`\n", packet.PacketID)
		fmt.Fprintf(&builder, "- Hidden case: `%s`\n", mapping.CaseID)
		fmt.Fprintf(&builder, "- Expected relation: `%s`\n", mapping.ExpectedRelation)
		fmt.Fprintf(&builder, "- Unit: `%s`\n", mapping.Unit)
		fmt.Fprintf(&builder, "- Task requirement digest: `%s`\n\n", packet.TaskRequirementDigest)
		fmt.Fprintln(&builder, "### Complete task requirement")
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, fencedUntrusted(packet.TaskRequirement))

		if packet.Unit == UnitCandidatePairOrders {
			if err := renderCandidateOrderAtlas(&builder, packet, mapping); err != nil {
				return "", err
			}
		} else {
			if err := renderTrajectoryPairAtlas(&builder, packet, mapping); err != nil {
				return "", err
			}
		}
		renderPilotAtlasDecisionMatrix(&builder, packet.Unit)
	}

	fmt.Fprintln(&builder, "\n## Atlas completion boundary")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "- [ ] Open the complete owner workbook for every packet before deciding information sufficiency or task context.")
	fmt.Fprintln(&builder, "- [ ] Treat every structural review flag as a question, never as an automatic failure or pass.")
	fmt.Fprintln(&builder, "- [ ] Record decisions only through the sealed owner-inspection workflow.")
	fmt.Fprintln(&builder, "- [ ] Keep human-study and external-action status unchanged until their separate gates complete.")
	return builder.String(), nil
}

func renderTrajectoryPairAtlas(builder *strings.Builder, packet BlindPacket, mapping PrivateMapping) error {
	originalMapping, transformedMapping, err := trajectoryPairMappings(mapping)
	if err != nil {
		return err
	}
	original, err := packetEvidenceForMapping(packet, originalMapping)
	if err != nil {
		return err
	}
	transformed, err := packetEvidenceForMapping(packet, transformedMapping)
	if err != nil {
		return err
	}
	change := buildPilotLineChange(original.Content, transformed.Content, pilotChangeAtlasContextLines)
	if len(change.OriginalChanged) == 0 && len(change.TransformedChanged) == 0 {
		return errors.New("relation pilot change atlas found no rendered change in a trajectory pair")
	}

	fmt.Fprintln(builder, "\n### Hidden logical alignment")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Logical side | Visible side | Content digest | Retained/source events | Omitted events |")
	fmt.Fprintln(builder, "|---|---|---|---:|---:|")
	fmt.Fprintf(builder, "| `original` | `%s` | `%s` | %d/%d | %d |\n", originalMapping.VisiblePosition, original.ContentDigest, original.RetainedEvents, original.SourceEvents, original.OmittedEvents)
	fmt.Fprintf(builder, "| `transformed` | `%s` | `%s` | %d/%d | %d |\n", transformedMapping.VisiblePosition, transformed.ContentDigest, transformed.RetainedEvents, transformed.SourceEvents, transformed.OmittedEvents)

	fmt.Fprintln(builder, "\n### Complete rendered change")
	fmt.Fprintln(builder)
	fmt.Fprintf(builder, "- Common prefix lines: %d\n", change.CommonPrefixLines)
	fmt.Fprintf(builder, "- Original-only changed lines: %d\n", len(change.OriginalChanged))
	fmt.Fprintf(builder, "- Transformed-only changed lines: %d\n", len(change.TransformedChanged))
	fmt.Fprintf(builder, "- Common suffix lines: %d\n", change.CommonSuffixLines)
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, fencedUntrusted(renderPilotLineChange(change)))

	fmt.Fprintln(builder, "\n### Structural review flags")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "- `all_non_common_rendered_lines_included=true`")
	fmt.Fprintf(builder, "- `paired_lineage_equal=%t`\n", originalMapping.RetainedLineageDigest == transformedMapping.RetainedLineageDigest)
	if mapping.Family == "causally_independent_event_reorder" && pilotLineChangeContains(change, "[Event reference:") {
		fmt.Fprintln(builder, "- `manual_causal_reference_review_required=true`: the reordered window contains an event-reference wrapper; verify that presentation order does not imply a changed dependency.")
	} else {
		fmt.Fprintln(builder, "- `manual_causal_reference_review_required=false`")
	}
	if original.OmittedEvents > 0 || transformed.OmittedEvents > 0 {
		fmt.Fprintln(builder, "- `full_workbook_required_for_omitted_context=true`")
	} else {
		fmt.Fprintln(builder, "- `full_workbook_required_for_omitted_context=false`")
	}
	return nil
}

func renderCandidateOrderAtlas(builder *strings.Builder, packet BlindPacket, mapping PrivateMapping) error {
	originalMappings, transformedMappings, err := candidateOrderMappings(mapping)
	if err != nil {
		return err
	}
	fmt.Fprintln(builder, "\n### Exact candidate reversal")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Source candidate | Original visible position/index | Transformed visible position/index | Content digest | Exact content match |")
	fmt.Fprintln(builder, "|---:|---|---|---|---|")
	for sourceIndex := 1; sourceIndex <= len(originalMappings); sourceIndex++ {
		originalMapping := originalMappings[sourceIndex-1]
		transformedMapping := transformedMappings[len(transformedMappings)-sourceIndex]
		if originalMapping.SourceCandidateIndex != sourceIndex || transformedMapping.SourceCandidateIndex != sourceIndex {
			return errors.New("relation pilot change atlas candidate mappings do not form an exact reversal")
		}
		original, evidenceErr := packetEvidenceForMapping(packet, originalMapping)
		if evidenceErr != nil {
			return evidenceErr
		}
		transformed, evidenceErr := packetEvidenceForMapping(packet, transformedMapping)
		if evidenceErr != nil {
			return evidenceErr
		}
		exact := original.ContentDigest == transformed.ContentDigest && original.Content == transformed.Content
		if !exact {
			return errors.New("relation pilot change atlas candidate reversal changed candidate content")
		}
		fmt.Fprintf(builder, "| %d | `%s`/%d | `%s`/%d | `%s` | `true` |\n", sourceIndex, originalMapping.VisiblePosition, originalMapping.VisibleEvidenceIndex, transformedMapping.VisiblePosition, transformedMapping.VisibleEvidenceIndex, original.ContentDigest)
	}
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "- `candidate_sequence_exact_reversal=true`")
	fmt.Fprintln(builder, "- `candidate_content_exactly_preserved=true`")
	fmt.Fprintln(builder, "- `semantic_preference_preserved=requires_owner_decision`")
	return nil
}

func trajectoryPairMappings(mapping PrivateMapping) (PrivateEvidenceMapping, PrivateEvidenceMapping, error) {
	if len(mapping.EvidenceMappings) != 2 {
		return PrivateEvidenceMapping{}, PrivateEvidenceMapping{}, errors.New("relation pilot change atlas trajectory pair requires two mappings")
	}
	var original, transformed PrivateEvidenceMapping
	for _, evidence := range mapping.EvidenceMappings {
		switch evidence.LogicalSide {
		case LogicalOriginal:
			original = evidence
		case LogicalTransformed:
			transformed = evidence
		}
	}
	if original.SlotID == "" || transformed.SlotID == "" {
		return PrivateEvidenceMapping{}, PrivateEvidenceMapping{}, errors.New("relation pilot change atlas trajectory pair lacks a logical side")
	}
	return original, transformed, nil
}

func candidateOrderMappings(mapping PrivateMapping) ([]PrivateEvidenceMapping, []PrivateEvidenceMapping, error) {
	if len(mapping.EvidenceMappings) != 4 {
		return nil, nil, errors.New("relation pilot change atlas candidate reversal requires four mappings")
	}
	var original, transformed []PrivateEvidenceMapping
	for _, evidence := range mapping.EvidenceMappings {
		switch evidence.LogicalSide {
		case LogicalOriginal:
			original = append(original, evidence)
		case LogicalTransformed:
			transformed = append(transformed, evidence)
		}
	}
	sort.Slice(original, func(left, right int) bool {
		return original[left].VisibleEvidenceIndex < original[right].VisibleEvidenceIndex
	})
	sort.Slice(transformed, func(left, right int) bool {
		return transformed[left].VisibleEvidenceIndex < transformed[right].VisibleEvidenceIndex
	})
	if len(original) != 2 || len(transformed) != 2 {
		return nil, nil, errors.New("relation pilot change atlas candidate reversal lacks two complete orderings")
	}
	return original, transformed, nil
}

func packetEvidenceForMapping(packet BlindPacket, mapping PrivateEvidenceMapping) (PacketEvidence, error) {
	for _, side := range packet.Sides {
		if side.Position != mapping.VisiblePosition {
			continue
		}
		index := mapping.VisibleEvidenceIndex - 1
		if index < 0 || index >= len(side.Evidence) {
			return PacketEvidence{}, errors.New("relation pilot change atlas mapping evidence index is outside the packet")
		}
		evidence := side.Evidence[index]
		if evidence.SlotID != mapping.SlotID || evidence.ContentDigest != mapping.ContentDigest {
			return PacketEvidence{}, errors.New("relation pilot change atlas mapping does not bind packet evidence")
		}
		return evidence, nil
	}
	return PacketEvidence{}, errors.New("relation pilot change atlas mapping references a missing visible side")
}

func buildPilotLineChange(original, transformed string, contextLines int) pilotLineChange {
	originalLines := pilotContentLines(original)
	transformedLines := pilotContentLines(transformed)
	prefix := 0
	for prefix < len(originalLines) && prefix < len(transformedLines) && originalLines[prefix] == transformedLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(originalLines)-prefix && suffix < len(transformedLines)-prefix && originalLines[len(originalLines)-1-suffix] == transformedLines[len(transformedLines)-1-suffix] {
		suffix++
	}
	prefixContextStart := max(0, prefix-contextLines)
	suffixContextCount := min(contextLines, suffix)
	return pilotLineChange{
		CommonPrefixLines:  prefix,
		CommonSuffixLines:  suffix,
		OriginalChanged:    append([]string(nil), originalLines[prefix:len(originalLines)-suffix]...),
		TransformedChanged: append([]string(nil), transformedLines[prefix:len(transformedLines)-suffix]...),
		PrefixContext:      append([]string(nil), originalLines[prefixContextStart:prefix]...),
		SuffixContext:      append([]string(nil), originalLines[len(originalLines)-suffix:len(originalLines)-suffix+suffixContextCount]...),
	}
}

func pilotContentLines(content string) []string {
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func renderPilotLineChange(change pilotLineChange) string {
	var builder strings.Builder
	for _, line := range change.PrefixContext {
		fmt.Fprintf(&builder, "  %s\n", line)
	}
	for _, line := range change.OriginalChanged {
		fmt.Fprintf(&builder, "- %s\n", line)
	}
	for _, line := range change.TransformedChanged {
		fmt.Fprintf(&builder, "+ %s\n", line)
	}
	for _, line := range change.SuffixContext {
		fmt.Fprintf(&builder, "  %s\n", line)
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

func pilotLineChangeContains(change pilotLineChange, value string) bool {
	for _, lines := range [][]string{change.OriginalChanged, change.TransformedChanged} {
		for _, line := range lines {
			if strings.Contains(line, value) {
				return true
			}
		}
	}
	return false
}

func renderPilotAtlasDecisionMatrix(builder *strings.Builder, unit UnitType) {
	fmt.Fprintln(builder, "\n### Owner decision matrix")
	fmt.Fprintln(builder)
	for _, dimension := range []string{"task_context", "evidence_alignment", "transformation_isolation", "information_sufficiency", "blinding_integrity", "rubric_applicability", "redistribution_boundary"} {
		fmt.Fprintf(builder, "- `%s`: [ ] passed  [ ] failed  [ ] indeterminate\n", dimension)
	}
	if unit == UnitCandidatePairOrders {
		fmt.Fprintln(builder, "- `candidate_order`: [ ] passed  [ ] failed  [ ] indeterminate")
	} else {
		fmt.Fprintln(builder, "- `candidate_order`: `not_applicable`")
	}
	fmt.Fprintln(builder, "- This atlas records no selection; seal decisions through `relation pilot-inspection` only.")
}
