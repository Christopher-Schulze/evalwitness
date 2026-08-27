package registry

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
)

const MaxIntakePayloadBytes = 1 << 20

var (
	zipMagic  = []byte{'P', 'K', 0x03, 0x04}
	gzipMagic = []byte{0x1f, 0x8b}
	ustarMark = []byte("ustar")
)

func RejectArchivePayload(name string, body []byte) error {
	if err := rejectUnsafeIntakeText("archive_name", name); err != nil {
		return err
	}
	if hasSuffixFold(name, ".zip") || hasSuffixFold(name, ".tar") || hasSuffixFold(name, ".tar.gz") || hasSuffixFold(name, ".tgz") {
		return fmt.Errorf("registry: archive payloads are not intake entries")
	}
	if bytes.HasPrefix(body, zipMagic) || bytes.HasPrefix(body, gzipMagic) || bytes.Contains(body[:min(len(body), 512)], ustarMark) {
		return fmt.Errorf("registry: archive bytes are not intake entries")
	}
	return nil
}

func RejectUnboundedAllocation(size int) error {
	if size < 0 || size > MaxIntakePayloadBytes {
		return fmt.Errorf("registry: payload exceeds %d bytes", MaxIntakePayloadBytes)
	}
	return nil
}

func VerifyDetachedEd25519(publicKey ed25519.PublicKey, message, signature []byte) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("registry: Ed25519 public key length is invalid")
	}
	if !ed25519.Verify(publicKey, message, signature) {
		return fmt.Errorf("registry: intake signature is invalid")
	}
	return nil
}

func hasSuffixFold(name, suffix string) bool {
	if len(name) < len(suffix) {
		return false
	}
	return equalFoldASCII(name[len(name)-len(suffix):], suffix)
}

func equalFoldASCII(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := 0; i < len(left); i++ {
		a, b := left[i], right[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}
