package safety

import (
	"regexp"
	"strings"
)

type TextMatch struct {
	Rule  string
	Start int
	End   int
}

type secretPattern struct {
	rule       string
	expression *regexp.Regexp
}

var secretPatterns = []secretPattern{
	{rule: "openai_key", expression: regexp.MustCompile(`(?i)\bsk-[a-zA-Z0-9_-]{20,}`)},
	{rule: "authorization_bearer", expression: regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-]{12,}`)},
	{rule: "authorization_header", expression: regexp.MustCompile(`(?i)Authorization\s*[:=]\s*(Basic|Bearer)\s+[^\s,;]+`)},
	{rule: "credential_assignment", expression: regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|refresh[_-]?token|password|passwd|secret)\b\s*[:=]\s*["']?[^"'\s,;&]{4,}`)},
	{rule: "cookie_header", expression: regexp.MustCompile(`(?i)\b(Set-Cookie|Cookie)\s*:\s*[^\r\n]+`)},
	{rule: "aws_access_key", expression: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{rule: "github_token", expression: regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`)},
	{rule: "slack_token", expression: regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)},
	{rule: "jwt", expression: regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)},
	{rule: "quoted_password", expression: regexp.MustCompile(`(?i)(["']?password["']?\s*[:=]\s*["'])[^"']+(["'])`)},
	{rule: "private_key", expression: regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`)},
}

func FindSecretPatterns(text string) []TextMatch {
	return findSecretPatternBytes([]byte(text))
}

func findSecretPatternBytes(data []byte) []TextMatch {
	var matches []TextMatch
	for _, pattern := range secretPatterns {
		for _, position := range pattern.expression.FindAllIndex(data, -1) {
			matches = append(matches, TextMatch{Rule: pattern.rule, Start: position[0], End: position[1]})
		}
	}
	return matches
}

func RedactSecretPatterns(text string) string {
	redacted := text
	for _, pattern := range secretPatterns {
		redacted = pattern.expression.ReplaceAllString(redacted, "[REDACTED]")
	}
	return redacted
}

func IsSensitiveFieldName(name string) bool {
	normalized := strings.Map(func(value rune) rune {
		if value >= 'a' && value <= 'z' || value >= '0' && value <= '9' {
			return value
		}
		if value >= 'A' && value <= 'Z' {
			return value + ('a' - 'A')
		}
		return -1
	}, name)
	for _, suffix := range []string{"authorization", "apikey", "password", "passwd", "secret", "credential", "cookie"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	switch normalized {
	case "token", "accesstoken", "refreshtoken", "idtoken", "session", "sessionid":
		return true
	default:
		return false
	}
}

func SecretsFromEnvironment(environment []string) []string {
	secrets := make([]string, 0)
	seen := map[string]struct{}{}
	for _, assignment := range environment {
		name, value, found := strings.Cut(assignment, "=")
		if !found || !IsSensitiveFieldName(name) || len(value) < 4 {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		secrets = append(secrets, value)
	}
	return secrets
}
