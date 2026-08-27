package study

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

func Lock(manifest Manifest, actor string) (Record, error) {
	if strings.TrimSpace(actor) == "" {
		return Record{}, errors.New("locking actor is required")
	}
	if err := manifest.Validate(); err != nil {
		return Record{}, err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return Record{}, err
	}
	var snapshot Manifest
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return Record{}, fmt.Errorf("snapshot study manifest: %w", err)
	}
	digestBytes := sha256.Sum256(encoded)
	digest := hex.EncodeToString(digestBytes[:])
	locked := LockedStudy{
		SchemaVersion:  LockedSchemaVersion,
		StudyID:        "study-" + digest,
		Manifest:       snapshot,
		ManifestDigest: digest,
	}
	record := Record{
		SchemaVersion: RecordSchemaVersion,
		Study:         locked,
		State:         StateLocked,
		Events: []Event{{
			From: StateDraft, To: StateLocked, At: manifest.Identity.LockedAt.UTC(),
			Actor: strings.TrimSpace(actor), Reason: "manifest validated and locked",
		}},
	}
	if err := record.seal(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func Transition(record Record, to State, at time.Time, actor, reason string, attestationDigests []string) (Record, error) {
	if err := record.Validate(); err != nil {
		return Record{}, err
	}
	if err := validateTransition(record.State, to); err != nil {
		return Record{}, err
	}
	if at.IsZero() || !at.After(record.Events[len(record.Events)-1].At) {
		return Record{}, errors.New("transition time must be later than the previous event")
	}
	if missing(actor, reason) {
		return Record{}, errors.New("transition actor and reason are required")
	}
	if to == StateAuthorized {
		if len(attestationDigests) != len(record.Study.Manifest.Arms) {
			return Record{}, errors.New("authorization requires one current attestation digest per arm")
		}
		locked := make([]string, len(record.Study.Manifest.Arms))
		for index, arm := range record.Study.Manifest.Arms {
			locked[index] = arm.AttestationDigest
		}
		if !sameStringSet(attestationDigests, locked) {
			return Record{}, errors.New("authorization attestations do not match the locked arms")
		}
		for _, providerPlan := range record.Study.Manifest.Providers {
			if !at.Before(providerPlan.AttestationExpiresAt) {
				return Record{}, fmt.Errorf("authorization occurs after the attestation for arm %q expires", providerPlan.ArmID)
			}
		}
	} else if len(attestationDigests) != 0 {
		return Record{}, errors.New("only authorization transitions may carry attestation digests")
	}
	for _, digest := range attestationDigests {
		if !validDigest(digest) {
			return Record{}, errors.New("transition attestation digest must be SHA-256")
		}
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return Record{}, fmt.Errorf("snapshot study record: %w", err)
	}
	var next Record
	if err := json.Unmarshal(encoded, &next); err != nil {
		return Record{}, fmt.Errorf("snapshot study record: %w", err)
	}
	next.Events = append(next.Events, Event{
		From: record.State, To: to, At: at.UTC(), Actor: strings.TrimSpace(actor), Reason: strings.TrimSpace(reason),
		AttestationDigests: sortedCopy(attestationDigests), PreviousRecordDigest: record.RecordDigest,
	})
	next.State = to
	next.RecordDigest = ""
	if err := next.seal(); err != nil {
		return Record{}, err
	}
	return next, nil
}

func ManifestDigest(manifest Manifest) (string, error) {
	if err := manifest.Validate(); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode study manifest: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (locked LockedStudy) Validate() error {
	if locked.SchemaVersion != LockedSchemaVersion || !validDigest(locked.ManifestDigest) {
		return errors.New("locked study schema or manifest digest is invalid")
	}
	digest, err := ManifestDigest(locked.Manifest)
	if err != nil {
		return err
	}
	if locked.ManifestDigest != digest || locked.StudyID != "study-"+digest {
		return errors.New("locked study identity does not match the canonical manifest")
	}
	return nil
}

func (record Record) Validate() error {
	if record.SchemaVersion != RecordSchemaVersion || !validState(record.State) || len(record.Events) == 0 || !validDigest(record.RecordDigest) {
		return errors.New("study record schema, state, events, or digest is invalid")
	}
	if err := record.Study.Validate(); err != nil {
		return err
	}
	if record.Events[0].From != StateDraft || record.Events[0].To != StateLocked || record.Events[0].At.IsZero() || !record.Events[0].At.UTC().Equal(record.Study.Manifest.Identity.LockedAt.UTC()) {
		return errors.New("study record must begin with the manifest lock event")
	}
	if record.Events[0].PreviousRecordDigest != "" || len(record.Events[0].AttestationDigests) != 0 {
		return errors.New("manifest lock event cannot contain a prior digest or route attestations")
	}
	current := StateLocked
	previousTime := record.Events[0].At
	for index, event := range record.Events {
		if missing(event.Actor, event.Reason) || event.At.IsZero() {
			return fmt.Errorf("lifecycle event %d is incomplete", index)
		}
		if index == 0 {
			continue
		}
		if event.From != current || !event.At.After(previousTime) {
			return fmt.Errorf("lifecycle event %d breaks state or time ordering", index)
		}
		if err := validateTransition(event.From, event.To); err != nil {
			return err
		}
		if !validDigest(event.PreviousRecordDigest) {
			return fmt.Errorf("lifecycle event %d lacks a prior immutable record digest", index)
		}
		prefix := Record{
			SchemaVersion: record.SchemaVersion, Study: record.Study, State: event.From,
			Events: append([]Event(nil), record.Events[:index]...),
		}
		expectedPrevious, err := prefix.digest()
		if err != nil {
			return err
		}
		if event.PreviousRecordDigest != expectedPrevious {
			return fmt.Errorf("lifecycle event %d does not bind the preceding immutable record", index)
		}
		if event.To == StateAuthorized {
			lockedAttestations := make([]string, len(record.Study.Manifest.Arms))
			for armIndex, arm := range record.Study.Manifest.Arms {
				lockedAttestations[armIndex] = arm.AttestationDigest
			}
			if !sameStringSet(event.AttestationDigests, lockedAttestations) {
				return fmt.Errorf("lifecycle event %d does not bind every locked arm attestation", index)
			}
			for _, providerPlan := range record.Study.Manifest.Providers {
				if !event.At.Before(providerPlan.AttestationExpiresAt) {
					return fmt.Errorf("lifecycle event %d occurs after the attestation for arm %q expires", index, providerPlan.ArmID)
				}
			}
		} else if len(event.AttestationDigests) != 0 {
			return fmt.Errorf("lifecycle event %d carries attestations outside authorization", index)
		}
		current = event.To
		previousTime = event.At
	}
	if current != record.State {
		return errors.New("record state does not match its event chain")
	}
	expected, err := record.digest()
	if err != nil {
		return err
	}
	if record.RecordDigest != expected {
		return errors.New("study record digest is invalid")
	}
	return nil
}

func (record *Record) seal() error {
	digest, err := record.digest()
	if err != nil {
		return err
	}
	record.RecordDigest = digest
	return record.Validate()
}

func (record Record) digest() (string, error) {
	record.RecordDigest = ""
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("encode study record: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validateTransition(from, to State) error {
	allowed := false
	switch from {
	case StateLocked:
		allowed = to == StateAuthorized || to == StateWithdrawn
	case StateAuthorized:
		allowed = to == StateRunning || to == StateFailed || to == StateWithdrawn
	case StateRunning:
		allowed = to == StateComplete || to == StateFailed || to == StateWithdrawn
	}
	if !allowed {
		return fmt.Errorf("study lifecycle transition %q -> %q is not permitted", from, to)
	}
	return nil
}

func validState(state State) bool {
	return slices.Contains([]State{StateDraft, StateLocked, StateAuthorized, StateRunning, StateComplete, StateFailed, StateWithdrawn}, state)
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	left = sortedCopy(left)
	right = sortedCopy(right)
	return slices.Equal(left, right)
}
