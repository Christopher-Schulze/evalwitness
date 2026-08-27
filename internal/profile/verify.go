package profile

import "fmt"

// Verify checks digest and validation offline; no network. Digest is required.
func Verify(p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.Digest == "" {
		return fmt.Errorf("profile: digest empty")
	}
	d, err := p.DigestValue()
	if err != nil {
		return err
	}
	if p.Digest != d {
		return fmt.Errorf("profile: digest mismatch expected %s got %s", d, p.Digest)
	}
	return nil
}
