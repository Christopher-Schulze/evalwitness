package claim

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
)

type evidenceSet struct {
	records  map[string]capsule.ComponentRecord
	payloads map[string][]byte
}

func newEvidenceSet(manifest capsule.Manifest, payloads map[string][]byte) evidenceSet {
	records := make(map[string]capsule.ComponentRecord, len(manifest.Components))
	for _, record := range manifest.Components {
		records[record.Name] = record
	}
	return evidenceSet{records: records, payloads: payloads}
}

func evaluateExpression(expression Expression, evidence evidenceSet) (ExactValue, error) {
	if err := expression.Validate(); err != nil {
		return ExactValue{}, err
	}
	switch expression.Operation {
	case OperationComponentExists:
		operand := expression.Operands[0]
		record, found := evidence.records[operand.Component]
		return BooleanValue(found && record.TypeID == operand.TypeID), nil
	case OperationPointerEquals:
		return evaluateOperand(expression.Operands[0], evidence)
	case OperationCount:
		value, err := resolveOperand(expression.Operands[0], evidence)
		if err != nil {
			return ExactValue{}, err
		}
		count, err := claimContainerLength(value)
		if err != nil {
			return ExactValue{}, err
		}
		return NumberValue(strconv.Itoa(count)), nil
	case OperationSum:
		total := new(big.Rat)
		for _, operand := range expression.Operands {
			value, err := resolveOperand(operand, evidence)
			if err != nil {
				return ExactValue{}, err
			}
			if err := addNumericValue(total, value); err != nil {
				return ExactValue{}, err
			}
		}
		return NumberValue(total.RatString()), nil
	case OperationRatio, OperationDifference:
		left, err := evaluateNumericOperand(expression.Operands[0], evidence)
		if err != nil {
			return ExactValue{}, err
		}
		right, err := evaluateNumericOperand(expression.Operands[1], evidence)
		if err != nil {
			return ExactValue{}, err
		}
		result := new(big.Rat)
		if expression.Operation == OperationRatio {
			if right.Sign() == 0 {
				return ExactValue{}, errors.New("claim ratio denominator is zero")
			}
			result.Quo(left, right)
		} else {
			result.Sub(left, right)
		}
		return NumberValue(result.RatString()), nil
	case OperationAllEqual:
		first, err := evaluateOperand(expression.Operands[0], evidence)
		if err != nil {
			return ExactValue{}, err
		}
		for _, operand := range expression.Operands[1:] {
			current, err := evaluateOperand(operand, evidence)
			if err != nil {
				return ExactValue{}, err
			}
			if !current.Equal(first) {
				return BooleanValue(false), nil
			}
		}
		return BooleanValue(true), nil
	default:
		return ExactValue{}, fmt.Errorf("unsupported claim operation %q", expression.Operation)
	}
}

func evaluateOperand(operand Operand, evidence evidenceSet) (ExactValue, error) {
	value, err := resolveOperand(operand, evidence)
	if err != nil {
		return ExactValue{}, err
	}
	return exactValueFromJSON(value)
}

func evaluateNumericOperand(operand Operand, evidence evidenceSet) (*big.Rat, error) {
	value, err := resolveOperand(operand, evidence)
	if err != nil {
		return nil, err
	}
	number, err := exactNumberFromRaw(value)
	if err != nil {
		return nil, fmt.Errorf("claim operand %q is not numeric", operand.Pointer)
	}
	return number, nil
}

func resolveOperand(operand Operand, evidence evidenceSet) (json.RawMessage, error) {
	record, found := evidence.records[operand.Component]
	if !found {
		return nil, fmt.Errorf("claim evidence component %q is missing", operand.Component)
	}
	if record.TypeID != operand.TypeID {
		return nil, fmt.Errorf("claim evidence component %q has type %q, want %q", operand.Component, record.TypeID, operand.TypeID)
	}
	raw, found := evidence.payloads[record.Payload.Digest]
	if !found {
		return nil, fmt.Errorf("claim evidence payload %q is missing", record.Payload.Digest)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode claim evidence component %q: %w", operand.Component, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("claim evidence component %q contains trailing JSON", operand.Component)
	}
	return resolveJSONPointer(value, operand.Pointer)
}

func resolveJSONPointer(value json.RawMessage, pointer string) (json.RawMessage, error) {
	if pointer == "" {
		return value, nil
	}
	if !validJSONPointer(pointer) {
		return nil, errors.New("claim JSON pointer is invalid")
	}
	current := value
	for _, rawToken := range strings.Split(pointer[1:], "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(rawToken, "~1", "/"), "~0", "~")
		trimmed := bytes.TrimSpace(current)
		if len(trimmed) == 0 {
			return nil, fmt.Errorf("claim JSON pointer %q traverses empty evidence", pointer)
		}
		switch trimmed[0] {
		case '{':
			var object map[string]json.RawMessage
			if err := json.Unmarshal(trimmed, &object); err != nil {
				return nil, fmt.Errorf("decode claim object at %q: %w", pointer, err)
			}
			next, found := object[token]
			if !found {
				return nil, fmt.Errorf("claim JSON pointer %q is missing object key %q", pointer, token)
			}
			current = next
		case '[':
			var array []json.RawMessage
			if err := json.Unmarshal(trimmed, &array); err != nil {
				return nil, fmt.Errorf("decode claim array at %q: %w", pointer, err)
			}
			if token == "" || token == "-" || (len(token) > 1 && token[0] == '0') {
				return nil, fmt.Errorf("claim JSON pointer %q has invalid array index %q", pointer, token)
			}
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(array) {
				return nil, fmt.Errorf("claim JSON pointer %q array index %q is out of range", pointer, token)
			}
			current = array[index]
		default:
			return nil, fmt.Errorf("claim JSON pointer %q traverses scalar evidence", pointer)
		}
	}
	return current, nil
}

func exactValueFromJSON(value json.RawMessage) (ExactValue, error) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return ExactValue{}, errors.New("claim expression resolved to empty JSON")
	}
	switch trimmed[0] {
	case 'n':
		if !bytes.Equal(trimmed, []byte("null")) {
			return ExactValue{}, errors.New("claim expression resolved to invalid null JSON")
		}
		return NullValue(), nil
	case 't', 'f':
		var boolean bool
		if err := json.Unmarshal(trimmed, &boolean); err != nil {
			return ExactValue{}, err
		}
		return BooleanValue(boolean), nil
	case '"':
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return ExactValue{}, err
		}
		return StringValue(text), nil
	case '{', '[':
		return ExactValue{}, errors.New("claim expression resolved to a non-scalar JSON value")
	default:
		rational, err := exactNumberFromRaw(trimmed)
		if err != nil {
			return ExactValue{}, err
		}
		return NumberValue(rational.RatString()), nil
	}
}

func claimContainerLength(value json.RawMessage) (int, error) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return 0, errors.New("claim count operand is empty")
	}
	switch trimmed[0] {
	case '[':
		var array []json.RawMessage
		if err := json.Unmarshal(trimmed, &array); err != nil {
			return 0, err
		}
		return len(array), nil
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return 0, err
		}
		return len(object), nil
	default:
		return 0, errors.New("claim count operand is not an array or object")
	}
}

func addNumericValue(total *big.Rat, value json.RawMessage) error {
	trimmed := bytes.TrimSpace(value)
	values := []json.RawMessage{trimmed}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return err
		}
	}
	for _, item := range values {
		rational, err := exactNumberFromRaw(item)
		if err != nil {
			return errors.New("claim sum contains a non-numeric value")
		}
		total.Add(total, rational)
	}
	return nil
}

func exactNumberFromRaw(value json.RawMessage) (*big.Rat, error) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 || (trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9')) {
		return nil, errors.New("claim JSON value is not numeric")
	}
	return exactJSONNumber(string(trimmed))
}

func exactJSONNumber(value string) (*big.Rat, error) {
	if value == "" || strings.HasPrefix(value, "+") {
		return nil, fmt.Errorf("claim JSON number %q is invalid", value)
	}
	mantissa := value
	exponent := 0
	if index := strings.IndexAny(value, "eE"); index >= 0 {
		mantissa = value[:index]
		parsed, err := strconv.Atoi(value[index+1:])
		if err != nil || parsed < -10_000 || parsed > 10_000 {
			return nil, fmt.Errorf("claim JSON number %q has an invalid exponent", value)
		}
		exponent = parsed
	}
	negative := strings.HasPrefix(mantissa, "-")
	unsigned := strings.TrimPrefix(mantissa, "-")
	parts := strings.Split(unsigned, ".")
	if len(parts) > 2 || len(parts[0]) == 0 || len(parts) == 2 && len(parts[1]) == 0 {
		return nil, fmt.Errorf("claim JSON number %q has an invalid mantissa", value)
	}
	digits := strings.Join(parts, "")
	for _, character := range digits {
		if character < '0' || character > '9' {
			return nil, fmt.Errorf("claim JSON number %q contains a non-digit", value)
		}
	}
	numerator, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, fmt.Errorf("claim JSON number %q is invalid", value)
	}
	if negative {
		numerator.Neg(numerator)
	}
	fractionDigits := 0
	if len(parts) == 2 {
		fractionDigits = len(parts[1])
	}
	power := exponent - fractionDigits
	denominator := big.NewInt(1)
	if power >= 0 {
		numerator.Mul(numerator, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(power)), nil))
	} else {
		denominator.Exp(big.NewInt(10), big.NewInt(int64(-power)), nil)
	}
	return new(big.Rat).SetFrac(numerator, denominator), nil
}
