package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	ServedIdentityPolicyExactObserved    = "exact_observed"
	ServedIdentityPolicyExactObservedSet = "exact_observed_set"
)

func ValidateServedIdentityPolicy(policy, expectedModel string, expectedModels []string) error {
	switch policy {
	case "":
		if expectedModel != "" || len(expectedModels) != 0 {
			return errors.New("served identity expectations require a policy")
		}
		return nil
	case ServedIdentityPolicyExactObserved:
		if err := validateServedModel(expectedModel); err != nil {
			return fmt.Errorf("expected served model: %w", err)
		}
		if len(expectedModels) != 0 {
			return errors.New("exact_observed requires one expected served model and no expected served-model set")
		}
		return nil
	case ServedIdentityPolicyExactObservedSet:
		if expectedModel != "" {
			return errors.New("exact_observed_set requires expected_served_model to be empty")
		}
		if len(expectedModels) < 2 {
			return errors.New("exact_observed_set requires at least two expected served models")
		}
		if !slices.IsSorted(expectedModels) {
			return errors.New("expected served models must be sorted")
		}
		for index, model := range expectedModels {
			if err := validateServedModel(model); err != nil {
				return fmt.Errorf("expected served model %d: %w", index, err)
			}
			if index > 0 && model == expectedModels[index-1] {
				return fmt.Errorf("expected served model %q is duplicated", model)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported served identity policy %q", policy)
	}
}

func MatchServedIdentity(policy, expectedModel string, expectedModels []string, observedModel string) error {
	if err := ValidateServedIdentityPolicy(policy, expectedModel, expectedModels); err != nil {
		return err
	}
	if err := validateServedModel(observedModel); err != nil {
		return fmt.Errorf("observed served model: %w", err)
	}
	switch policy {
	case ServedIdentityPolicyExactObserved:
		if observedModel != expectedModel {
			return fmt.Errorf("observed served model %q differs from expected model %q", observedModel, expectedModel)
		}
	case ServedIdentityPolicyExactObservedSet:
		if !slices.Contains(expectedModels, observedModel) {
			return fmt.Errorf("observed served model %q is outside the expected set %q", observedModel, expectedModels)
		}
	default:
		return errors.New("served identity policy is required")
	}
	return nil
}

func validateServedModel(model string) error {
	if model == "" || model != strings.TrimSpace(model) || len(model) > 250 || !utf8.ValidString(model) || strings.IndexFunc(model, unicode.IsControl) >= 0 {
		return errors.New("must be non-empty, trimmed UTF-8 without control characters and at most 250 bytes")
	}
	return nil
}
