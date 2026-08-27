package conformance

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const maximumAttestationBytes = 2 * 1024 * 1024

var ErrAttestationNotFound = errors.New("capability attestation not found")

type Store struct {
	root *safety.CacheRoot
}

func OpenStore(cacheDirectory string) (*Store, error) {
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return nil, err
	}
	root, err := safety.CreateCacheRoot(policy, cacheDirectory)
	if err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func OpenExistingStore(cacheDirectory string) (*Store, error) {
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return nil, err
	}
	root, err := safety.OpenCacheRoot(policy, cacheDirectory)
	if err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Save(attestation Attestation) error {
	if s == nil || s.root == nil {
		return errors.New("attestation store is not open")
	}
	if err := attestation.ValidateIntegrity(); err != nil {
		return err
	}
	path, err := attestationPath(attestation.Identity.RouteConfigDigest, attestation.Contract.ContractDigest)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(attestation, "", "  ")
	if err != nil {
		return fmt.Errorf("encode capability attestation: %w", err)
	}
	if len(encoded) > maximumAttestationBytes {
		return errors.New("capability attestation exceeds the storage limit")
	}
	return s.root.PublishSensitive(path, encoded)
}

func (s *Store) Load(routeConfigDigest, contractDigest string, now time.Time, studyManifestDigest string) (Attestation, RouteState, ExpirationReason, error) {
	if s == nil || s.root == nil {
		return Attestation{}, "", "", errors.New("attestation store is not open")
	}
	path, err := attestationPath(routeConfigDigest, contractDigest)
	if err != nil {
		return Attestation{}, "", "", err
	}
	encoded, err := s.root.ReadSensitive(path, maximumAttestationBytes)
	if errors.Is(err, os.ErrNotExist) {
		return Attestation{}, "", "", ErrAttestationNotFound
	}
	if err != nil {
		return Attestation{}, "", "", err
	}
	var attestation Attestation
	if err := json.Unmarshal(encoded, &attestation); err != nil {
		return Attestation{}, "", "", fmt.Errorf("decode capability attestation: %w", err)
	}
	if err := attestation.ValidateIntegrity(); err != nil {
		return Attestation{}, "", "", err
	}
	if attestation.Identity.RouteConfigDigest != routeConfigDigest || attestation.Contract.ContractDigest != contractDigest {
		return Attestation{}, "", "", errors.New("capability attestation identity does not match its storage key")
	}
	state, reason := attestation.EffectiveState(now, routeConfigDigest, contractDigest, studyManifestDigest)
	return attestation, state, reason, nil
}

func attestationPath(routeConfigDigest, contractDigest string) (string, error) {
	routeConfigDigest = strings.TrimSpace(routeConfigDigest)
	contractDigest = strings.TrimSpace(contractDigest)
	if !validSHA256(routeConfigDigest) || !validSHA256(contractDigest) {
		return "", errors.New("attestation storage identity must contain SHA-256 digests")
	}
	return filepath.Join("attestations", routeConfigDigest, contractDigest+".json"), nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
