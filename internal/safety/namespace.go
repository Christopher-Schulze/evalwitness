package safety

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	RouteNamespaceSchema       = 1
	RouteNamespacePrefix       = "route-"
	RouteIdentityFileName      = "identity.json"
	MaxExternalIdentifierBytes = 4096
	maxRouteIdentityBytes      = 16 * 1024
)

type RouteNamespace struct {
	ID       string `json:"namespace_id"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type RouteNamespaceMetadata struct {
	SchemaVersion int    `json:"schema_version"`
	NamespaceID   string `json:"namespace_id"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
}

func NewRouteNamespace(provider, model string) (RouteNamespace, error) {
	if !validExternalIdentifier(provider) || !validExternalIdentifier(model) {
		return RouteNamespace{}, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
	return RouteNamespace{
		ID:       routeNamespaceID(provider, model),
		Provider: provider,
		Model:    model,
	}, nil
}

func (n RouteNamespace) Valid() bool {
	return validExternalIdentifier(n.Provider) && validExternalIdentifier(n.Model) &&
		n.ID == routeNamespaceID(n.Provider, n.Model)
}

func (n RouteNamespace) Directory() string {
	return filepath.Join("routes", n.ID)
}

func (n RouteNamespace) IdentityPath() string {
	return filepath.Join(n.Directory(), RouteIdentityFileName)
}

func (n RouteNamespace) Metadata() RouteNamespaceMetadata {
	return RouteNamespaceMetadata{
		SchemaVersion: RouteNamespaceSchema,
		NamespaceID:   n.ID,
		Provider:      n.Provider,
		Model:         n.Model,
	}
}

func (m RouteNamespaceMetadata) Valid() bool {
	namespace, err := NewRouteNamespace(m.Provider, m.Model)
	return err == nil && m.SchemaVersion == RouteNamespaceSchema && m.NamespaceID == namespace.ID
}

func (r *CacheRoot) EnsureRouteIdentity(namespace RouteNamespace) error {
	if r == nil || !namespace.Valid() {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
	if err := r.verifyRouteIdentity(namespace); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, err := json.Marshal(namespace.Metadata())
	if err != nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationWrite, Cause: err}
	}
	if err := r.PublishSensitive(namespace.IdentityPath(), raw); err != nil {
		return err
	}
	return r.verifyRouteIdentity(namespace)
}

func (r *CacheRoot) VerifyRouteIdentity(namespace RouteNamespace) error {
	if r == nil || !namespace.Valid() {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
	return r.verifyRouteIdentity(namespace)
}

func (r *CacheRoot) verifyRouteIdentity(namespace RouteNamespace) error {
	raw, err := r.ReadSensitive(namespace.IdentityPath(), maxRouteIdentityBytes)
	if err != nil {
		return err
	}
	var metadata RouteNamespaceMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil || !metadata.Valid() || metadata != namespace.Metadata() {
		return &Error{Kind: ErrorNameCollision, Operation: OperationRead, Path: namespace.IdentityPath(), Cause: err}
	}
	return nil
}

func validExternalIdentifier(identifier string) bool {
	return identifier != "" && len(identifier) <= MaxExternalIdentifierBytes && utf8.ValidString(identifier)
}

func routeNamespaceID(provider, model string) string {
	payload := make([]byte, 16+len(provider)+len(model))
	binary.BigEndian.PutUint64(payload[:8], uint64(len(provider)))
	providerEnd := 8 + copy(payload[8:], provider)
	binary.BigEndian.PutUint64(payload[providerEnd:providerEnd+8], uint64(len(model)))
	copy(payload[providerEnd+8:], model)
	digest := sha256.Sum256(payload)
	return RouteNamespacePrefix + hex.EncodeToString(digest[:])
}

func IsSafeNamespaceID(identifier string) bool {
	if !strings.HasPrefix(identifier, RouteNamespacePrefix) {
		return false
	}
	digest := strings.TrimPrefix(identifier, RouteNamespacePrefix)
	if len(digest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
