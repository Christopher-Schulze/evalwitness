package profile

import "strings"

// SpecSchema is the profile JSON schema identifier.
const SpecSchema = "evalwitness.profile.v1.schema"

// SpecCanonical is the spec.md anchor.
const SpecCanonical = "VerifierReliabilityProfile"

// IsSpecFlushed checks spec contains profile schema.
func IsSpecFlushed(doc string) bool {
	return strings.Contains(doc, SpecCanonical) && strings.Contains(doc, SpecSchema)
}
