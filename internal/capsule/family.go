package capsule

import (
	"context"
	"errors"
	"fmt"
)

type Package struct {
	Registry *Registry
	Manifest Manifest
	Payloads map[string][]byte
}

func VerifyPackageFamily(ctx context.Context, child Package, parents []Package, options VerificationOptions) (VerificationReport, error) {
	if ctx == nil || child.Registry == nil {
		return VerificationReport{}, errors.New("capsule family verification requires context and a child registry")
	}
	report, err := VerifyPackage(ctx, child.Registry, child.Manifest, child.Payloads, options)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("verify child capsule: %w", err)
	}
	parentByID := make(map[string]Package, len(parents))
	for index, parent := range parents {
		if parent.Registry == nil {
			return VerificationReport{}, fmt.Errorf("parent capsule %d has no registry", index)
		}
		if _, err := VerifyPackage(ctx, parent.Registry, parent.Manifest, parent.Payloads, VerificationOptions{}); err != nil {
			return VerificationReport{}, fmt.Errorf("verify parent capsule %d: %w", index, err)
		}
		if _, duplicate := parentByID[parent.Manifest.CapsuleID]; duplicate {
			return VerificationReport{}, fmt.Errorf("capsule family repeats parent %q", parent.Manifest.CapsuleID)
		}
		parentByID[parent.Manifest.CapsuleID] = parent
	}
	if len(parentByID) != len(child.Manifest.ParentCapsules) {
		return VerificationReport{}, errors.New("capsule family parent set differs from the child manifest")
	}
	for _, reference := range child.Manifest.ParentCapsules {
		if _, found := parentByID[reference.CapsuleID]; !found {
			return VerificationReport{}, fmt.Errorf("declared parent capsule %q is unavailable", reference.CapsuleID)
		}
	}
	baseDigest := child.Registry.Document().BaseRegistryDigest
	if baseDigest != "" {
		matches := 0
		for _, parent := range parents {
			if parent.Registry.Digest() == baseDigest {
				matches++
			}
		}
		if matches != 1 {
			return VerificationReport{}, errors.New("capsule extension does not resolve exactly one base registry")
		}
	}
	for _, component := range child.Manifest.Components {
		for _, reference := range component.Parents {
			if reference.Resolution == ParentInternal || reference.CapsuleID == "" {
				continue
			}
			parent := parentByID[reference.CapsuleID]
			resolved, found := packageComponentByID(parent.Manifest, reference.ComponentID)
			if !found {
				return VerificationReport{}, fmt.Errorf("component %q has unresolved external parent %q", component.Name, reference.ComponentID)
			}
			if resolved.TypeID != reference.TypeID || resolved.Role != reference.Role || resolved.Visibility != reference.Visibility {
				return VerificationReport{}, fmt.Errorf("component %q external parent metadata differs from capsule %q", component.Name, reference.CapsuleID)
			}
		}
	}
	return report, nil
}

func packageComponentByID(manifest Manifest, componentID string) (ComponentRecord, bool) {
	for _, component := range manifest.Components {
		if component.ComponentID == componentID {
			return component, true
		}
	}
	return ComponentRecord{}, false
}
