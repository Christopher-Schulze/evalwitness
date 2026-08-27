package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

var (
	defaultLogger *slog.Logger
	levelVar      = new(slog.LevelVar)
	initOnce      sync.Once
	secretMu      sync.RWMutex
	secrets       []string
)

type redactingHandler struct {
	wrapped slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.wrapped.Enabled(ctx, lvl)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = Redact(r.Message)
	attrs := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, redactAttr(a))
		return true
	})
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	nr.AddAttrs(attrs...)
	return h.wrapped.Handle(ctx, nr)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for index, attr := range attrs {
		redacted[index] = redactAttr(attr)
	}
	return &redactingHandler{wrapped: h.wrapped.WithAttrs(redacted)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{wrapped: h.wrapped.WithGroup(Redact(name))}
}

func Redact(s string) string {
	out := safety.RedactSecretPatterns(s)
	secretMu.RLock()
	keys := secrets
	secretMu.RUnlock()
	for _, k := range keys {
		if k != "" {
			out = strings.ReplaceAll(out, k, "[REDACTED]")
		}
	}
	return out
}

func RegisterSecret(s string) {
	if s == "" {
		return
	}
	secretMu.Lock()
	secrets = append(secrets, s)
	secretMu.Unlock()
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Init(level string) {
	initOnce.Do(func() {
		levelVar.Set(parseLevel(level))
		base := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: levelVar})
		defaultLogger = slog.New(&redactingHandler{wrapped: base})
		slog.SetDefault(defaultLogger)
	})
	levelVar.Set(parseLevel(level))
}

// SetLevel updates the runtime log level. Safe to call from any goroutine.
func SetLevel(level string) {
	levelVar.Set(parseLevel(level))
}

func L() *slog.Logger {
	if defaultLogger == nil {
		Init("info")
	}
	return defaultLogger
}
