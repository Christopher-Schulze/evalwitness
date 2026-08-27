package log

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const maxStructuredRedactionDepth = 16

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Key = Redact(attr.Key)
	attr.Value = redactSlogValue(attr.Key, attr.Value.Resolve(), 0)
	return attr
}

func redactSlogValue(key string, value slog.Value, depth int) slog.Value {
	if safety.IsSensitiveFieldName(key) {
		return slog.StringValue("[REDACTED]")
	}
	if depth >= maxStructuredRedactionDepth {
		return slog.StringValue("[REDACTED:DEPTH_LIMIT]")
	}
	switch value.Kind() {
	case slog.KindString:
		return slog.StringValue(Redact(value.String()))
	case slog.KindGroup:
		attrs := value.Group()
		redacted := make([]slog.Attr, len(attrs))
		for index, attr := range attrs {
			attr.Key = Redact(attr.Key)
			attr.Value = redactSlogValue(attr.Key, attr.Value.Resolve(), depth+1)
			redacted[index] = attr
		}
		return slog.GroupValue(redacted...)
	case slog.KindAny:
		return slog.AnyValue(redactStructuredAny(value.Any(), depth+1))
	default:
		return value
	}
}

func redactStructuredAny(value any, depth int) any {
	if value == nil {
		return nil
	}
	if depth >= maxStructuredRedactionDepth {
		return "[REDACTED:DEPTH_LIMIT]"
	}
	switch typed := value.(type) {
	case string:
		return Redact(typed)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr, float32, float64, json.Number:
		return typed
	case error:
		return Redact(typed.Error())
	case http.Header:
		return redactHeader(typed, depth+1)
	case url.URL:
		return redactURL(typed)
	case *url.URL:
		if typed == nil {
			return nil
		}
		return redactURL(*typed)
	case map[string]any:
		return redactStringMap(typed, depth+1)
	case map[string]string:
		values := make(map[string]any, len(typed))
		for key, item := range typed {
			values[key] = item
		}
		return redactStringMap(values, depth+1)
	case []any:
		return redactSlice(typed, depth+1)
	case []string:
		values := make([]any, len(typed))
		for index, item := range typed {
			values[index] = item
		}
		return redactSlice(values, depth+1)
	default:
		return redactJSONOrString(typed, depth+1)
	}
}

func redactHeader(header http.Header, depth int) map[string]any {
	redacted := make(map[string]any, len(header))
	for key, values := range header {
		if safety.IsSensitiveFieldName(key) {
			redacted[Redact(key)] = "[REDACTED]"
			continue
		}
		items := make([]any, len(values))
		for index, value := range values {
			items[index] = Redact(value)
		}
		redacted[Redact(key)] = redactSlice(items, depth+1)
	}
	return redacted
}

func redactURL(value url.URL) string {
	if value.User != nil {
		username := Redact(value.User.Username())
		if _, hasPassword := value.User.Password(); hasPassword {
			value.User = url.UserPassword(username, "[REDACTED]")
		} else {
			value.User = url.User(username)
		}
	}
	query := value.Query()
	for key, values := range query {
		for index, item := range values {
			if safety.IsSensitiveFieldName(key) {
				values[index] = "[REDACTED]"
			} else {
				values[index] = Redact(item)
			}
		}
		query[key] = values
	}
	value.RawQuery = query.Encode()
	value.Path = Redact(value.Path)
	value.RawPath = ""
	value.Fragment = Redact(value.Fragment)
	return Redact(value.String())
}

func redactStringMap(values map[string]any, depth int) map[string]any {
	redacted := make(map[string]any, len(values))
	for key, value := range values {
		redactedKey := Redact(key)
		if safety.IsSensitiveFieldName(key) {
			redacted[redactedKey] = "[REDACTED]"
			continue
		}
		redacted[redactedKey] = redactStructuredAny(value, depth+1)
	}
	return redacted
}

func redactSlice(values []any, depth int) []any {
	redacted := make([]any, len(values))
	for index, value := range values {
		redacted[index] = redactStructuredAny(value, depth+1)
	}
	return redacted
}

func redactJSONOrString(value any, depth int) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return Redact(fmt.Sprintf("%v", value))
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return Redact(string(raw))
	}
	return redactStructuredAny(decoded, depth+1)
}
