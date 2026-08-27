package profile

import "strings"

// DocsPath is the documentation path for profile.
const DocsPath = "docs/documentation.md#reliability-profile"

// DocsSection anchors profile flush verification.
const DocsSection = "Reliability profile"

// IsDocsFlushed checks marker presence in documentation text.
func IsDocsFlushed(doc string) bool {
	return strings.Contains(doc, DocsSection) && strings.Contains(doc, "internal/profile")
}
