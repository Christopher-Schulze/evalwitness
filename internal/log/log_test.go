package log

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestRedactMasksPatternMatches(t *testing.T) {
	// The pattern set exists so that a key pasted into a prompt, an error body
	// or a URL never reaches stderr. Each case is a shape a provider or a user
	// actually produces.
	cases := []struct {
		name  string
		in    string
		leaks string
	}{
		{"openai style key", "using sk-proj-EXAMPLEKEYNOTREAL0000000000 now", "sk-proj-EXAMPLE"},
		{"bearer header", "Authorization: Bearer abcdefghijklmnopqrstuvwxyz012345", "abcdefghijklmnop"},
		{"basic header", "Authorization: Basic dXNlcjp0aW55", "dXNlcjp0aW55"},
		{"short query secret", "https://example.invalid/path?api_key=tiny&ok=1", "tiny"},
		{"cookie header", "Cookie: session=short-value", "short-value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Redact(tc.in)
			if strings.Contains(out, tc.leaks) {
				t.Fatalf("secret survived redaction: %q", out)
			}
			if !strings.Contains(out, "[REDACTED]") {
				t.Fatalf("no redaction marker in %q", out)
			}
		})
	}
}

func TestRegisterSecretMasksAnExactValue(t *testing.T) {
	// A key that matches no pattern is still a key. Registering it is what the
	// CLI does at startup with whatever the config resolved.
	const key = "not-a-recognisable-shape-9f3a"
	if got := Redact("token " + key); !strings.Contains(got, key) {
		t.Fatal("test precondition failed: the value already matches a pattern")
	}
	RegisterSecret(key)
	out := Redact("token " + key)
	if strings.Contains(out, key) {
		t.Fatalf("registered secret survived redaction: %q", out)
	}
}

func TestRegisterSecretIgnoresEmpty(t *testing.T) {
	// An unset key resolves to the empty string. Registering it would replace
	// every empty position in every message, which is both useless and slow.
	RegisterSecret("")
	if out := Redact("nothing to hide"); out != "nothing to hide" {
		t.Fatalf("empty secret altered output: %q", out)
	}
}

func TestHandlerRedactsMessageAndStringAttrs(t *testing.T) {
	const key = "handler-secret-value-77b2"
	RegisterSecret(key)

	var buf bytes.Buffer
	h := &redactingHandler{wrapped: slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})}
	slog.New(h).Info("request failed for "+key, "url", "https://example.invalid?token="+key, "attempts", 3)

	out := buf.String()
	if strings.Contains(out, key) {
		t.Fatalf("secret reached the log output: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("no redaction marker: %s", out)
	}
	// Non-string attributes pass through untouched; redacting them would corrupt
	// numbers and booleans for no benefit.
	if !strings.Contains(out, "attempts=3") {
		t.Fatalf("non-string attribute was altered: %s", out)
	}
}

func TestHandlerRedactsThroughWithAttrsAndWithGroup(t *testing.T) {
	// slog wraps handlers when a logger carries context. A wrapper that forgets
	// to re-wrap would silently stop redacting exactly where structured logging
	// is used most.
	const key = "grouped-secret-value-31c8"
	RegisterSecret(key)

	var buf bytes.Buffer
	base := &redactingHandler{wrapped: slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})}
	logger := slog.New(base).With("authorization", key).WithGroup("request-" + key)
	logger.Info("sending", "auth", key)

	if out := buf.String(); strings.Contains(out, key) {
		t.Fatalf("secret leaked through WithAttrs/WithGroup: %s", out)
	}
}

type secretLogValue struct {
	secret string
}

func (v secretLogValue) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("password", v.secret),
		slog.Group("nested", slog.String("message", "echo "+v.secret)),
	)
}

func TestHandlerRedactsNestedValuesErrorsURLsHeadersAndLogValuers(t *testing.T) {
	const registered = "nested-registered-secret-88f1"
	RegisterSecret(registered)
	requestURL, err := url.Parse("https://user:short-password@example.invalid/path?api_key=tiny&query=" + registered)
	if err != nil {
		t.Fatal(err)
	}
	headers := http.Header{
		"Authorization": {"Basic tiny"},
		"Cookie":        {"session=short"},
		"X-Trace":       {registered},
	}
	payload := map[string]any{
		"credential": "short-credential",
		"nested": []any{
			map[string]any{"message": "contains " + registered},
			42,
			true,
		},
	}

	var buf bytes.Buffer
	handler := &redactingHandler{wrapped: slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})}
	slog.New(handler).Info("structured",
		"error", errors.New("provider echoed "+registered),
		"url", requestURL,
		"headers", headers,
		"payload", payload,
		"valuer", secretLogValue{secret: registered},
	)

	out := buf.String()
	for _, leaked := range []string{registered, "tiny", "short-password", "short-credential", "session=short"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("structured value leaked %q: %s", leaked, out)
		}
	}
	if !strings.Contains(out, "42") || !strings.Contains(out, "true") {
		t.Fatalf("non-secret primitives were lost: %s", out)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Fatalf("redaction marker missing: %s", out)
	}
}

func TestSensitiveFieldDetectionDoesNotHideUsageMetrics(t *testing.T) {
	for _, field := range []string{"input_tokens", "output_tokens", "cached_tokens", "top_logprobs_max"} {
		if safety.IsSensitiveFieldName(field) {
			t.Fatalf("metric %q was classified as a secret", field)
		}
	}
	for _, field := range []string{"Authorization", "X-API-Key", "db_password", "client-secret", "refresh_token", "Set-Cookie"} {
		if !safety.IsSensitiveFieldName(field) {
			t.Fatalf("secret field %q was not classified", field)
		}
	}
}

func TestHandlerEnabledDelegatesToWrapped(t *testing.T) {
	var buf bytes.Buffer
	h := &redactingHandler{wrapped: slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})}
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug reported enabled under a warn-level handler")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("error reported disabled under a warn-level handler")
	}
}

func TestParseLevelAndSetLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":    slog.LevelDebug,
		"DEBUG":    slog.LevelDebug,
		"info":     slog.LevelInfo,
		"warn":     slog.LevelWarn,
		"error":    slog.LevelError,
		"":         slog.LevelInfo,
		"nonsense": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Fatalf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}

	Init("error")
	if L().Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info enabled after Init(error)")
	}
	SetLevel("debug")
	if !L().Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug disabled after SetLevel(debug)")
	}
	SetLevel("info")
}

func TestRedactIsSafeUnderConcurrentRegistration(t *testing.T) {
	// Secrets are registered during startup while worker goroutines are already
	// logging. The guard around the slice has to hold under -race.
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			RegisterSecret(string(rune('a'+i)) + "-concurrent-secret-value")
		})
		wg.Go(func() {
			for range 50 {
				_ = Redact("message with a-concurrent-secret-value inside")
			}
		})
	}
	wg.Wait()
}
