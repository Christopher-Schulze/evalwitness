package profile

import "fmt"

// Build creates a validated Profile with deterministic digest.
func Build(identity, protocolVersion, routeScope string, dimensions []Dimension) (Profile, error) {
	p := Profile{
		SchemaVersion:   SchemaVersion,
		Identity:        identity,
		ProtocolVersion: protocolVersion,
		RouteScope:      routeScope,
		Dimensions:      dimensions,
	}
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	d, err := p.DigestValue()
	if err != nil {
		return Profile{}, fmt.Errorf("profile: digest: %w", err)
	}
	p.Digest = d
	return p, nil
}
