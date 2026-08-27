package lineage

import (
	"errors"
	"slices"
	"sort"
)

type RetentionLossKind string

const (
	RetentionRequiredFieldLoss   RetentionLossKind = "required_field_loss"
	RetentionDecisiveChannelLoss RetentionLossKind = "decisive_channel_loss"
)

type RetentionLoss struct {
	Layer  string            `json:"layer"`
	Kind   RetentionLossKind `json:"kind"`
	Name   string            `json:"name"`
	Reason string            `json:"reason"`
}

type RetentionAnalysis struct {
	CandidateDigest         string          `json:"candidate_digest"`
	RequiredFields          []string        `json:"required_fields"`
	DecisiveChannels        []string        `json:"decisive_channels"`
	SurvivingChannels       []string        `json:"surviving_channels"`
	TruncatedRequiredFields []string        `json:"truncated_required_fields"`
	Losses                  []RetentionLoss `json:"losses"`
	Complete                bool            `json:"complete"`
	Digest                  string          `json:"digest"`
}

func AnalyzeRetention(candidate LineageCandidate, requiredFields, decisiveChannels []string) (RetentionAnalysis, error) {
	if err := candidate.Validate(); err != nil {
		return RetentionAnalysis{}, err
	}
	fields := append([]string(nil), requiredFields...)
	channels := append([]string(nil), decisiveChannels...)
	if err := validateSortedUnique("retention required fields", fields, 1); err != nil {
		return RetentionAnalysis{}, err
	}
	if err := validateSortedUnique("retention decisive channels", channels, 1); err != nil {
		return RetentionAnalysis{}, err
	}
	lostFields := make(map[string]struct{}, len(fields))
	lostChannels := make(map[string]struct{}, len(channels))
	analysis := RetentionAnalysis{CandidateDigest: candidate.Header.Digest, RequiredFields: fields, DecisiveChannels: channels}
	for _, layer := range candidate.Layers {
		for _, field := range fields {
			if _, lost := lostFields[field]; lost {
				continue
			}
			reason := ""
			if !layer.StructuredPresence {
				reason = "structured_presence_lost"
			} else if !slices.Contains(layer.RequiredFields, field) {
				reason = "required_field_absent"
			}
			if reason != "" {
				analysis.Losses = append(analysis.Losses, RetentionLoss{Layer: layer.Layer, Kind: RetentionRequiredFieldLoss, Name: field, Reason: reason})
				analysis.TruncatedRequiredFields = append(analysis.TruncatedRequiredFields, field)
				lostFields[field] = struct{}{}
			}
		}
		for _, channel := range channels {
			if _, lost := lostChannels[channel]; lost {
				continue
			}
			reason := ""
			if !layer.SemanticSufficiency {
				reason = "semantic_sufficiency_lost"
			} else if !slices.Contains(layer.DecisiveChannels, channel) {
				reason = "decisive_channel_absent"
			}
			if reason != "" {
				analysis.Losses = append(analysis.Losses, RetentionLoss{Layer: layer.Layer, Kind: RetentionDecisiveChannelLoss, Name: channel, Reason: reason})
				lostChannels[channel] = struct{}{}
			}
		}
	}
	for _, channel := range channels {
		if _, lost := lostChannels[channel]; !lost {
			analysis.SurvivingChannels = append(analysis.SurvivingChannels, channel)
		}
	}
	sort.Strings(analysis.TruncatedRequiredFields)
	analysis.Complete = len(analysis.Losses) == 0
	var err error
	analysis.Digest, err = retentionAnalysisDigest(analysis)
	if err != nil {
		return RetentionAnalysis{}, err
	}
	return analysis, analysis.Validate()
}

func (analysis RetentionAnalysis) Validate() error {
	if !validDigest(analysis.CandidateDigest) || !validDigest(analysis.Digest) {
		return errors.New("retention analysis identity is invalid")
	}
	if err := validateSortedUnique("retention analysis required fields", analysis.RequiredFields, 1); err != nil {
		return err
	}
	if err := validateSortedUnique("retention analysis decisive channels", analysis.DecisiveChannels, 1); err != nil {
		return err
	}
	if len(analysis.SurvivingChannels) > 0 {
		if err := validateSortedUnique("retention analysis surviving channels", analysis.SurvivingChannels, 1); err != nil {
			return err
		}
	}
	if len(analysis.TruncatedRequiredFields) > 0 {
		if err := validateSortedUnique("retention analysis truncated fields", analysis.TruncatedRequiredFields, 1); err != nil {
			return err
		}
	}
	if analysis.Complete != (len(analysis.Losses) == 0) {
		return errors.New("retention analysis completeness contradicts its losses")
	}
	seen := make(map[string]struct{}, len(analysis.Losses))
	lostFields := make(map[string]struct{}, len(analysis.RequiredFields))
	lostChannels := make(map[string]struct{}, len(analysis.DecisiveChannels))
	layers := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	previousLayer := -1
	previousKind := -1
	previousName := ""
	for _, loss := range analysis.Losses {
		if missing(loss.Layer, loss.Name, loss.Reason) || !slices.Contains([]RetentionLossKind{RetentionRequiredFieldLoss, RetentionDecisiveChannelLoss}, loss.Kind) {
			return errors.New("retention analysis contains an invalid loss")
		}
		layerIndex := slices.Index(layers, loss.Layer)
		kindIndex := 0
		if loss.Kind == RetentionDecisiveChannelLoss {
			kindIndex = 1
		}
		if layerIndex < 0 || layerIndex < previousLayer || (layerIndex == previousLayer && (kindIndex < previousKind || (kindIndex == previousKind && loss.Name <= previousName))) {
			return errors.New("retention losses are not in canonical first-loss order")
		}
		previousLayer, previousKind, previousName = layerIndex, kindIndex, loss.Name
		key := string(loss.Kind) + "\x00" + loss.Name
		if _, duplicate := seen[key]; duplicate {
			return errors.New("retention analysis contains more than one first loss for a field or channel")
		}
		seen[key] = struct{}{}
		switch loss.Kind {
		case RetentionRequiredFieldLoss:
			if !slices.Contains(analysis.RequiredFields, loss.Name) || !slices.Contains([]string{"required_field_absent", "structured_presence_lost"}, loss.Reason) {
				return errors.New("retention analysis required-field loss is invalid")
			}
			lostFields[loss.Name] = struct{}{}
		case RetentionDecisiveChannelLoss:
			if !slices.Contains(analysis.DecisiveChannels, loss.Name) || !slices.Contains([]string{"decisive_channel_absent", "semantic_sufficiency_lost"}, loss.Reason) {
				return errors.New("retention analysis decisive-channel loss is invalid")
			}
			lostChannels[loss.Name] = struct{}{}
		}
	}
	if len(lostFields) != len(analysis.TruncatedRequiredFields) {
		return errors.New("retention analysis truncated fields do not match first losses")
	}
	for _, field := range analysis.TruncatedRequiredFields {
		if _, found := lostFields[field]; !found {
			return errors.New("retention analysis truncated fields do not match first losses")
		}
	}
	expectedSurvivors := make([]string, 0, len(analysis.DecisiveChannels)-len(lostChannels))
	for _, channel := range analysis.DecisiveChannels {
		if _, lost := lostChannels[channel]; !lost {
			expectedSurvivors = append(expectedSurvivors, channel)
		}
	}
	if !slices.Equal(analysis.SurvivingChannels, expectedSurvivors) {
		return errors.New("retention analysis surviving channels do not partition decisive channels")
	}
	expected, err := retentionAnalysisDigest(analysis)
	if err != nil {
		return err
	}
	if analysis.Digest != expected {
		return errors.New("retention analysis digest is invalid")
	}
	return nil
}

func retentionAnalysisDigest(analysis RetentionAnalysis) (string, error) {
	analysis.Digest = ""
	return digestJSON(analysis)
}
