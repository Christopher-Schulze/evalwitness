package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maximumSafeJSONInteger = int64(9_007_199_254_740_991)

func CanonicalMarshal(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode protocol value: %w", err)
	}
	return CanonicalizeJSON(raw)
}

func CanonicalizeJSON(raw []byte) ([]byte, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("protocol JSON must be UTF-8")
	}
	if err := validateEscapedUnicodeScalars(raw); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeCanonicalValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("unexpected trailing JSON token %v", token)
		}
		return nil, fmt.Errorf("decode trailing protocol JSON: %w", err)
	}
	var output bytes.Buffer
	if err := writeCanonicalValue(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func validateEscapedUnicodeScalars(raw []byte) error {
	inString := false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(raw) {
				continue
			}
			if raw[index+1] != 'u' {
				index++
				continue
			}
			value, ok := decodeHexQuad(raw, index+2)
			if !ok {
				continue
			}
			if value >= 0xDC00 && value <= 0xDFFF {
				return errors.New("protocol JSON contains an unpaired low surrogate")
			}
			if value >= 0xD800 && value <= 0xDBFF {
				if index+12 > len(raw) || raw[index+6] != '\\' || raw[index+7] != 'u' {
					return errors.New("protocol JSON contains an unpaired high surrogate")
				}
				low, lowOK := decodeHexQuad(raw, index+8)
				if !lowOK || low < 0xDC00 || low > 0xDFFF {
					return errors.New("protocol JSON contains an invalid surrogate pair")
				}
				index += 11
				continue
			}
			index += 5
		}
	}
	return nil
}

func decodeHexQuad(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	var value uint16
	for _, char := range raw[start : start+4] {
		value <<= 4
		switch {
		case char >= '0' && char <= '9':
			value += uint16(char - '0')
		case char >= 'a' && char <= 'f':
			value += uint16(char-'a') + 10
		case char >= 'A' && char <= 'F':
			value += uint16(char-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func Digest(value any) (string, error) {
	canonical, err := CanonicalMarshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func DigestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func DecodeStrict(raw []byte, destination any) error {
	if !utf8.Valid(raw) {
		return errors.New("protocol JSON must be UTF-8")
	}
	if _, err := CanonicalizeJSON(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode strict protocol object: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("protocol object contains trailing JSON")
	}
	return nil
}

func decodeCanonicalValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("protocol JSON exceeds 64-level depth limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode protocol JSON: %w", err)
	}
	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			object := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("decode protocol object key: %w", err)
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, errors.New("protocol object key is not a string")
				}
				if _, duplicate := object[key]; duplicate {
					return nil, fmt.Errorf("duplicate protocol object key %q", key)
				}
				value, err := decodeCanonicalValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
				return nil, errors.New("protocol object is not terminated")
			}
			return object, nil
		case '[':
			array := make([]any, 0)
			for decoder.More() {
				value, err := decodeCanonicalValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
				return nil, errors.New("protocol array is not terminated")
			}
			return array, nil
		default:
			return nil, fmt.Errorf("unexpected protocol delimiter %q", typed)
		}
	case json.Number:
		if err := validateCanonicalInteger(string(typed)); err != nil {
			return nil, err
		}
		return typed, nil
	case string, bool, nil:
		return typed, nil
	default:
		return nil, fmt.Errorf("unsupported protocol JSON value %T", typed)
	}
}

func writeCanonicalValue(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		output.WriteString(strconv.FormatBool(typed))
	case json.Number:
		output.WriteString(string(typed))
	case string:
		encoded, err := encodeJSONString(typed)
		if err != nil {
			return err
		}
		output.Write(encoded)
	case []any:
		output.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := writeCanonicalValue(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			encoded, err := encodeJSONString(key)
			if err != nil {
				return err
			}
			output.Write(encoded)
			output.WriteByte(':')
			if err := writeCanonicalValue(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("cannot canonicalize protocol value %T", typed)
	}
	return nil
}

func encodeJSONString(value string) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("encode protocol JSON string: %w", err)
	}
	return bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), nil
}

func validateCanonicalInteger(value string) error {
	if strings.ContainsAny(value, ".eE+") || value == "-0" {
		return fmt.Errorf("protocol JSON number %q is not a canonical integer", value)
	}
	integer, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return fmt.Errorf("protocol JSON number %q is invalid", value)
	}
	limit := big.NewInt(maximumSafeJSONInteger)
	if new(big.Int).Abs(integer).Cmp(limit) > 0 {
		return fmt.Errorf("protocol JSON integer %q exceeds interoperable range", value)
	}
	return nil
}

func parseCanonicalDecimal(value string) (*big.Rat, error) {
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "+eE/") {
		return nil, fmt.Errorf("decimal %q is not canonical", value)
	}
	if value == "-0" || strings.HasPrefix(value, "-0.") && strings.Trim(value[3:], "0") == "" {
		return nil, fmt.Errorf("decimal %q encodes negative zero", value)
	}
	unsigned := strings.TrimPrefix(value, "-")
	parts := strings.Split(unsigned, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts[0]) > 1 && parts[0][0] == '0' {
		return nil, fmt.Errorf("decimal %q is not canonical", value)
	}
	if len(parts) == 2 && (parts[1] == "" || strings.HasSuffix(parts[1], "0")) {
		return nil, fmt.Errorf("decimal %q has a non-canonical fraction", value)
	}
	for _, part := range parts {
		for _, char := range part {
			if char < '0' || char > '9' {
				return nil, fmt.Errorf("decimal %q contains a non-digit", value)
			}
		}
	}
	rational, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, fmt.Errorf("decimal %q is invalid", value)
	}
	return rational, nil
}
