package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var bonBaseEnvironment = []string{"HOME", "LANG", "LC_ALL", "PATH", "SHELL", "TERM", "TMPDIR", "USER"}

var errBonOutputLimit = errors.New("command output exceeds limit")

type boundedTailWriter struct {
	mu      sync.Mutex
	data    []byte
	limit   int
	dropped int64
}

type boundedRejectWriter struct {
	data  []byte
	limit int
}

func (writer *boundedRejectWriter) Write(value []byte) (int, error) {
	remaining := writer.limit - len(writer.data)
	if remaining <= 0 {
		return 0, errBonOutputLimit
	}
	if len(value) > remaining {
		writer.data = append(writer.data, value[:remaining]...)
		return remaining, errBonOutputLimit
	}
	writer.data = append(writer.data, value...)
	return len(value), nil
}

func newBoundedTailWriter(limit int) *boundedTailWriter {
	return &boundedTailWriter{limit: limit, data: make([]byte, 0, limit)}
}

func (writer *boundedTailWriter) Write(value []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	written := len(value)
	if writer.limit <= 0 {
		writer.dropped += int64(written)
		return written, nil
	}
	if len(value) >= writer.limit {
		writer.dropped += int64(len(writer.data) + len(value) - writer.limit)
		writer.data = append(writer.data[:0], value[len(value)-writer.limit:]...)
		return written, nil
	}
	overflow := len(writer.data) + len(value) - writer.limit
	if overflow > 0 {
		copy(writer.data, writer.data[overflow:])
		writer.data = writer.data[:len(writer.data)-overflow]
		writer.dropped += int64(overflow)
	}
	writer.data = append(writer.data, value...)
	return written, nil
}

func (writer *boundedTailWriter) Snapshot() ([]byte, int64) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]byte(nil), writer.data...), writer.dropped
}

func bonChildEnvironment(passNames []string, attempt int) ([]string, error) {
	names := append([]string(nil), bonBaseEnvironment...)
	for _, rawName := range passNames {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		if !environmentNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid --pass-env name %q", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	environment := make([]string, 0, len(names)+1)
	previous := ""
	for _, name := range names {
		if name == previous {
			continue
		}
		previous = name
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	environment = append(environment, fmt.Sprintf("EVALWITNESS_BON_ATTEMPT=%d", attempt))
	return environment, nil
}

func validateBonPassEnvironment(passNames []string) error {
	_, err := bonChildEnvironment(passNames, 0)
	return err
}

func validateBonCommandArguments(arguments, passNames []string) error {
	for index, argument := range arguments {
		if safety.RedactSecretPatterns(argument) != argument {
			return fmt.Errorf("agent command argument %d contains secret-like material; pass secrets only through --pass-env references", index)
		}
		for _, name := range passNames {
			secret, exists := os.LookupEnv(strings.TrimSpace(name))
			if exists && secret != "" && strings.Contains(argument, secret) {
				return fmt.Errorf("agent command argument %d embeds the value of %s; reference the environment variable instead", index, name)
			}
		}
	}
	return nil
}

func redactBonTranscript(value []byte, passNames []string) []byte {
	redacted := safety.RedactSecretPatterns(string(value))
	for _, name := range passNames {
		if secret, exists := os.LookupEnv(strings.TrimSpace(name)); exists && secret != "" {
			redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
		}
	}
	return []byte(redacted)
}

func processExitCode(runErr error) (int, error) {
	if runErr == nil {
		return 0, nil
	}
	var exitError interface{ ExitCode() int }
	if errors.As(runErr, &exitError) {
		return exitError.ExitCode(), nil
	}
	return 0, runErr
}
