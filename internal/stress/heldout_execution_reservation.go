package stress

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const (
	HeldOutExecutionReservationSchemaVersion = "evalwitness.stress-held-out-execution-reservation.v1"
	heldOutExecutionReservationStatus        = "reserved_execution_not_started"
	heldOutExecutionReservationExternalState = "authorized_not_started"
	heldOutExecutionReservationMaxBytes      = 64 << 10
)

type HeldOutExecutionReservation struct {
	SchemaVersion        string                               `json:"schema_version"`
	CanonicalPolicy      string                               `json:"canonical_policy"`
	PermitDigest         string                               `json:"permit_digest"`
	PreflightCapsuleID   string                               `json:"preflight_capsule_id"`
	Authority            HeldOutExecutionReservationAuthority `json:"authority"`
	ReservationKey       string                               `json:"reservation_key"`
	ReservedAt           string                               `json:"reserved_at"`
	PermitExpiresAt      string                               `json:"permit_expires_at"`
	Status               string                               `json:"status"`
	ExternalActionStatus string                               `json:"external_action_status"`
	ExecutionStarted     bool                                 `json:"execution_started"`
	ProviderCalls        int                                  `json:"provider_calls"`
	EmpiricalUnits       int                                  `json:"empirical_units"`
	NetworkPerformed     bool                                 `json:"network_performed"`
	ClaimBoundary        HeldOutCampaignClaimBoundary         `json:"claim_boundary"`
	Digest               string                               `json:"digest"`
}

type HeldOutExecutionReservationStore struct {
	root      *safety.CacheRoot
	authority HeldOutExecutionReservationAuthority
	now       func() time.Time
}

func NewHeldOutExecutionReservationStore(root *safety.CacheRoot) (*HeldOutExecutionReservationStore, error) {
	authority, err := NewHeldOutExecutionReservationAuthority(root)
	if err != nil {
		return nil, err
	}
	return &HeldOutExecutionReservationStore{root: root, authority: authority, now: time.Now}, nil
}

func (store *HeldOutExecutionReservationStore) Authority() HeldOutExecutionReservationAuthority {
	if store == nil {
		return HeldOutExecutionReservationAuthority{}
	}
	return store.authority
}

func (store *HeldOutExecutionReservationStore) Reserve(
	ctx context.Context,
	permit HeldOutExecutionPermit,
	preflight capsule.Package,
	privateRelation capsule.Package,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
	providedAuthorizationDigests []string,
) (HeldOutExecutionReservation, error) {
	if store == nil || store.root == nil || store.now == nil {
		return HeldOutExecutionReservation{}, errors.New("stress held-out execution reservation requires an initialized store")
	}
	verifiedAt := store.now().UTC()
	if err := VerifyHeldOutExecutionPermit(
		ctx, permit, preflight, privateRelation, admission, execution, evidence,
		providedAuthorizationDigests, store.authority, verifiedAt,
	); err != nil {
		return HeldOutExecutionReservation{}, err
	}
	reservedAt := store.now().UTC()
	if err := permit.ValidateAt(reservedAt); err != nil {
		return HeldOutExecutionReservation{}, err
	}
	return store.reserveVerified(permit, reservedAt)
}

func (store *HeldOutExecutionReservationStore) reserveVerified(
	permit HeldOutExecutionPermit,
	reservedAt time.Time,
) (HeldOutExecutionReservation, error) {
	if store == nil || store.root == nil || reservedAt.IsZero() {
		return HeldOutExecutionReservation{}, errors.New("stress held-out execution reservation requires a store and reservation time")
	}
	if permit.ReservationAuthority != store.authority {
		return HeldOutExecutionReservation{}, errors.New("stress held-out execution permit targets a different reservation authority")
	}
	if err := permit.ValidateAt(reservedAt); err != nil {
		return HeldOutExecutionReservation{}, err
	}
	value := HeldOutExecutionReservation{
		SchemaVersion: HeldOutExecutionReservationSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PermitDigest: permit.Digest, PreflightCapsuleID: permit.PreflightCapsuleID, Authority: store.authority,
		ReservationKey: heldOutExecutionReservationKey(permit.Digest), ReservedAt: formatHeldOutExecutionPermitTime(reservedAt),
		PermitExpiresAt: permit.ExpiresAt, Status: heldOutExecutionReservationStatus,
		ExternalActionStatus: heldOutExecutionReservationExternalState,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim:    heldOutExecutionReservationSupportedClaim,
			UnsupportedClaims: slices.Clone(heldOutExecutionReservationUnsupportedClaims),
		},
	}
	var err error
	value.Digest, err = heldOutExecutionReservationDigest(value)
	if err != nil {
		return HeldOutExecutionReservation{}, err
	}
	if err := value.ValidateAgainst(permit, reservedAt); err != nil {
		return HeldOutExecutionReservation{}, err
	}
	raw, err := EncodeIndented(value)
	if err != nil {
		return HeldOutExecutionReservation{}, err
	}
	if err := store.root.PublishSensitiveExclusive(filepath.FromSlash(value.ReservationKey), raw); err != nil {
		if existing, readErr := store.Load(permit); readErr == nil {
			return HeldOutExecutionReservation{}, fmt.Errorf("stress held-out execution permit was already reserved at %s with receipt %s", existing.ReservedAt, existing.Digest)
		}
		return HeldOutExecutionReservation{}, fmt.Errorf("reserve stress held-out execution permit atomically: %w", err)
	}
	return value, nil
}

func (store *HeldOutExecutionReservationStore) Load(permit HeldOutExecutionPermit) (HeldOutExecutionReservation, error) {
	if store == nil || store.root == nil {
		return HeldOutExecutionReservation{}, errors.New("stress held-out execution reservation requires a store")
	}
	if permit.ReservationAuthority != store.authority {
		return HeldOutExecutionReservation{}, errors.New("stress held-out execution permit targets a different reservation authority")
	}
	raw, err := store.root.ReadSensitive(
		filepath.FromSlash(heldOutExecutionReservationKey(permit.Digest)), heldOutExecutionReservationMaxBytes,
	)
	if err != nil {
		return HeldOutExecutionReservation{}, err
	}
	value, err := DecodeHeldOutExecutionReservation(bytes.NewReader(raw))
	if err != nil {
		return HeldOutExecutionReservation{}, err
	}
	reservedAt, err := parseHeldOutExecutionPermitTime(value.ReservedAt)
	if err != nil {
		return HeldOutExecutionReservation{}, err
	}
	if err := value.ValidateAgainst(permit, reservedAt); err != nil {
		return HeldOutExecutionReservation{}, err
	}
	return value, nil
}

func (value HeldOutExecutionReservation) Validate() error {
	if value.SchemaVersion != HeldOutExecutionReservationSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.PermitDigest) || !validDigest(value.PreflightCapsuleID) ||
		value.ReservationKey != heldOutExecutionReservationKey(value.PermitDigest) {
		return errors.New("stress held-out execution reservation identity is invalid")
	}
	if err := value.Authority.Validate(); err != nil {
		return err
	}
	reservedAt, reservationErr := parseHeldOutExecutionPermitTime(value.ReservedAt)
	expiresAt, expiryErr := parseHeldOutExecutionPermitTime(value.PermitExpiresAt)
	if reservationErr != nil || expiryErr != nil || !reservedAt.Before(expiresAt) {
		return errors.New("stress held-out execution reservation time window is invalid")
	}
	if value.Status != heldOutExecutionReservationStatus || value.ExternalActionStatus != heldOutExecutionReservationExternalState ||
		value.ExecutionStarted || value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkPerformed ||
		value.ClaimBoundary.SupportedClaim != heldOutExecutionReservationSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutExecutionReservationUnsupportedClaims) {
		return errors.New("stress held-out execution reservation fabricates execution, observation, or claims")
	}
	expected, err := heldOutExecutionReservationDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out execution reservation digest is invalid")
	}
	return nil
}

func (value HeldOutExecutionReservation) ValidateAgainst(permit HeldOutExecutionPermit, reservedAt time.Time) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if reservedAt.IsZero() || value.ReservedAt != formatHeldOutExecutionPermitTime(reservedAt) ||
		value.PermitDigest != permit.Digest || value.PreflightCapsuleID != permit.PreflightCapsuleID ||
		value.PermitExpiresAt != permit.ExpiresAt || value.Authority != permit.ReservationAuthority {
		return errors.New("stress held-out execution reservation differs from its exact permit or reservation event")
	}
	return permit.ValidateAt(reservedAt)
}

func DecodeHeldOutExecutionReservation(reader io.Reader) (HeldOutExecutionReservation, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, heldOutExecutionReservationMaxBytes+1))
	if err != nil {
		return HeldOutExecutionReservation{}, fmt.Errorf("read stress held-out execution reservation: %w", err)
	}
	if len(raw) > heldOutExecutionReservationMaxBytes {
		return HeldOutExecutionReservation{}, errors.New("stress held-out execution reservation exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutExecutionReservation
	if err := decoder.Decode(&value); err != nil {
		return HeldOutExecutionReservation{}, fmt.Errorf("decode stress held-out execution reservation: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutExecutionReservation{}, errors.New("stress held-out execution reservation has trailing JSON")
	}
	return value, value.Validate()
}

func validHeldOutReservationAuthorityID(value string) bool {
	if len(value) != 32 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func heldOutExecutionReservationKey(permitDigest string) string {
	return "held-out-execution-reservations/" + permitDigest + ".json"
}

func heldOutExecutionReservationDigest(value HeldOutExecutionReservation) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutExecutionReservationSupportedClaim = "the exact execution permit has at most one successful reservation within its bound owner-only reservation authority without claiming that execution started"

var heldOutExecutionReservationUnsupportedClaims = []string{
	"held-out execution started or completed",
	"provider response evidence",
	"empirical verifier reliability",
	"global distributed consensus outside the bound reservation authority",
	"replay prevention after owner deletion rollback or cloning of the reservation authority",
	"held-out run seal",
}
