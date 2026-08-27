package profile

import "fmt"

// CLIBuild builds a profile from provided dimensions; fails if dimensions empty or invalid.
func CLIBuild(identity, protocol, route string, dims []Dimension) (Profile, error) {
	if len(dims) == 0 {
		return Profile{}, fmt.Errorf("cli: dimensions empty")
	}
	return Build(identity, protocol, route, dims)
}

// CLIVerify verifies a profile; fails on empty digest or invalid.
func CLIVerify(p Profile) error {
	if err := Verify(p); err != nil {
		return fmt.Errorf("cli verify: %w", err)
	}
	return nil
}
