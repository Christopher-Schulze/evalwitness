package safety

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type UntrustedInputKind string

const (
	InputProtocolAdapter UntrustedInputKind = "protocol_adapter"
	InputTrace           UntrustedInputKind = "trace"
	InputAttribution     UntrustedInputKind = "attribution"
	InputCapsule         UntrustedInputKind = "capsule"
	InputPolicy          UntrustedInputKind = "policy"
	InputStaticReport    UntrustedInputKind = "static_report"
)

func (k UntrustedInputKind) Valid() bool {
	switch k {
	case InputProtocolAdapter, InputTrace, InputAttribution, InputCapsule, InputPolicy, InputStaticReport:
		return true
	default:
		return false
	}
}

type UntrustedInputLimits struct {
	MaxBytes        int64
	MaxDepth        int
	MaxTotalNodes   int
	MaxStringBytes  int
	MaxArrayItems   int
	MaxObjectFields int
	MaxMarkupBytes  int
	MaxLinks        int
}

func (l UntrustedInputLimits) Valid() bool {
	return l.MaxBytes > 0 && l.MaxDepth > 0 && l.MaxTotalNodes > 0 &&
		l.MaxStringBytes > 0 && l.MaxArrayItems > 0 && l.MaxObjectFields > 0 &&
		l.MaxMarkupBytes > 0 && l.MaxLinks > 0
}

func DefaultUntrustedInputLimits(kind UntrustedInputKind) (UntrustedInputLimits, error) {
	base := UntrustedInputLimits{
		MaxDepth: 32, MaxTotalNodes: 250_000, MaxStringBytes: 1 << 20,
		MaxArrayItems: 100_000, MaxObjectFields: 10_000,
		MaxMarkupBytes: 4 << 20, MaxLinks: 10_000,
	}
	switch kind {
	case InputProtocolAdapter:
		base.MaxBytes = 8 << 20
	case InputTrace:
		base.MaxBytes = 256 << 20
		base.MaxTotalNodes = 2_000_000
	case InputAttribution:
		base.MaxBytes = 64 << 20
	case InputCapsule:
		base.MaxBytes = 512 << 20
		base.MaxTotalNodes = 4_000_000
	case InputPolicy:
		base.MaxBytes = 4 << 20
		base.MaxTotalNodes = 100_000
	case InputStaticReport:
		base.MaxBytes = 64 << 20
		base.MaxTotalNodes = 1_000_000
	default:
		return UntrustedInputLimits{}, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
	return base, nil
}

func ValidateUntrustedJSON(raw []byte, limits UntrustedInputLimits) error {
	if !limits.Valid() || int64(len(raw)) > limits.MaxBytes {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Cause: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Cause: err}
	}
	state := untrustedJSONState{limits: limits}
	return state.validate(value, 1)
}

func ReadAndValidateUntrustedJSON(path string, limits UntrustedInputLimits) ([]byte, error) {
	if path == "" || !limits.Valid() {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	if !info.Mode().IsRegular() {
		return nil, &Error{Kind: ErrorUnsupportedFileType, Operation: OperationRead}
	}
	if info.Size() < 0 || info.Size() > limits.MaxBytes {
		return nil, &Error{Kind: ErrorResourceLimit, Operation: OperationRead}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	if err := ValidateUntrustedJSON(raw, limits); err != nil {
		return nil, err
	}
	return raw, nil
}

type untrustedJSONState struct {
	limits UntrustedInputLimits
	nodes  int
}

func (s *untrustedJSONState) validate(value any, depth int) error {
	s.nodes++
	if depth > s.limits.MaxDepth || s.nodes > s.limits.MaxTotalNodes {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
	}
	switch typed := value.(type) {
	case nil, bool, json.Number:
		return nil
	case string:
		return s.validateString(typed)
	case []any:
		if len(typed) > s.limits.MaxArrayItems {
			return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
		}
		for _, item := range typed {
			if err := s.validate(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(typed) > s.limits.MaxObjectFields {
			return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
		}
		for key, item := range typed {
			if err := s.validateString(key); err != nil {
				return err
			}
			if err := s.validate(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
}

func (s *untrustedJSONState) validateString(value string) error {
	if len(value) > s.limits.MaxStringBytes {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
	}
	return nil
}

type UntrustedControlSelection struct {
	Command            []string
	Environment        map[string]string
	WorkingDirectory   string
	NetworkDestination string
	OutputPath         string
	LiveMode           bool
}

func RejectUntrustedControlSelection(selection UntrustedControlSelection) error {
	if len(selection.Command) > 0 || len(selection.Environment) > 0 ||
		selection.WorkingDirectory != "" || selection.NetworkDestination != "" ||
		selection.OutputPath != "" || selection.LiveMode {
		return &Error{Kind: ErrorUntrustedControl, Operation: OperationValidate}
	}
	return nil
}

func ValidateUntrustedMarkup(markup string, limits UntrustedInputLimits) error {
	if !limits.Valid() || len(markup) > limits.MaxMarkupBytes {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationValidate}
	}
	return nil
}

func ValidateOfflineLink(raw string) error {
	if raw == "" || strings.Contains(raw, "\\") {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Opaque != "" {
		return &Error{Kind: ErrorUntrustedControl, Operation: OperationValidate, Cause: err}
	}
	if parsed.RawQuery != "" || strings.HasPrefix(parsed.Path, "/") {
		return &Error{Kind: ErrorUntrustedControl, Operation: OperationValidate}
	}
	clean := filepath.ToSlash(filepath.Clean(parsed.Path))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return &Error{Kind: ErrorContainmentViolation, Operation: OperationValidate}
	}
	return nil
}
