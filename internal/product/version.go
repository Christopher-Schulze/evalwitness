package product

import "strings"

// Version is the single source of truth for the EvalWitness product version.
// Release tags use the same value with a leading "v".
const Version = "0.2.0"

// ValidVersion reports whether value is a canonical SemVer 2.0.0 version
// without the tag-only "v" prefix.
func ValidVersion(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "v") {
		return false
	}
	coreAndPrerelease, build, hasBuild := strings.Cut(value, "+")
	if hasBuild && !validIdentifiers(build, false) || strings.Contains(build, "+") {
		return false
	}
	core, prerelease, hasPrerelease := strings.Cut(coreAndPrerelease, "-")
	if hasPrerelease && !validIdentifiers(prerelease, true) {
		return false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !validNumericIdentifier(part) {
			return false
		}
	}
	return true
}

func validIdentifiers(value string, rejectNumericLeadingZero bool) bool {
	parts := strings.Split(value, ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		numeric := true
		for _, character := range part {
			if character < '0' || character > '9' {
				numeric = false
			}
			if !asciiAlphaNumeric(character) && character != '-' {
				return false
			}
		}
		if rejectNumericLeadingZero && numeric && len(part) > 1 && part[0] == '0' {
			return false
		}
	}
	return true
}

func asciiAlphaNumeric(character rune) bool {
	return character >= '0' && character <= '9' ||
		character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z'
}

func validNumericIdentifier(value string) bool {
	if value == "" || len(value) > 1 && value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
