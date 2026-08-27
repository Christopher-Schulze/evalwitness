package stress

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDevelopmentChallengeIsSelfContainedAndAdversariallyVerified(t *testing.T) {
	root := filepath.Join("..", "..")
	first, err := BuildDevelopmentChallenge(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildDevelopmentChallenge(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("stress development challenge is not deterministic")
	}
	receipt, err := VerifyDevelopmentChallenge(first)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ExpectedCaseDigest != first.Expected.CaseDigest || receipt.ReproducedCaseDigest != first.Expected.CaseDigest ||
		receipt.FixturesVerified != 4 || receipt.OriginalLineUnits != 32 || receipt.FinalLineUnits != 2 ||
		receipt.ReductionAttempts != 53 || receipt.AcceptedReductions != 30 || receipt.FinalRejectionAttempts != 2 ||
		!receipt.ViolationReproduced || !receipt.OneMinimalReproduced || receipt.RepositoryRequired ||
		receipt.EmpiricalUnits != 0 || receipt.ProviderCalls != 0 || receipt.NetworkRequired || !receipt.ClaimBoundaryPreserved ||
		receipt.GuardCount != 7 || receipt.GuardsPassed != 7 || !receipt.Valid {
		t.Fatalf("stress development challenge receipt = %+v", receipt)
	}
	for _, guard := range receipt.Guards {
		if !guard.Passed || guard.ExpectedGuard != guard.ObservedGuard {
			t.Fatalf("stress development challenge guard = %+v", guard)
		}
	}

	encoded, err := EncodeIndented(first)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeDevelopmentChallenge(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, decoded) {
		t.Fatal("stress development challenge changed across strict JSON decoding")
	}
	encodedReceipt, err := EncodeIndented(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decodedReceipt, err := DecodeDevelopmentChallengeReceipt(bytes.NewReader(encodedReceipt))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(receipt, decodedReceipt) {
		t.Fatal("stress development challenge receipt changed across strict JSON decoding")
	}
	verification, err := VerifyDevelopmentChallengeReceipt(first, decodedReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if !verification.Valid || verification.ReceiptDigest != receipt.Digest || verification.ChallengeDigest != first.Digest ||
		verification.GuardCount != 7 || verification.GuardsPassed != 7 || verification.EmpiricalUnits != 0 ||
		verification.ProviderCalls != 0 || verification.NetworkRequired {
		t.Fatalf("stress development challenge receipt verification = %+v", verification)
	}
}

func TestDevelopmentChallengeRejectsSubstitutionAndMalformedDocuments(t *testing.T) {
	value, err := BuildDevelopmentChallenge(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("fixture bytes", func(t *testing.T) {
		tampered := value
		tampered.Fixtures = slices.Clone(value.Fixtures)
		raw, err := hex.DecodeString(tampered.Fixtures[1].ContentHex)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)-1] ^= 1
		tampered.Fixtures[1].ContentHex = hex.EncodeToString(raw)
		tampered.Digest, err = developmentChallengeDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDevelopmentChallenge(tampered); err == nil || !strings.Contains(err.Error(), "fixture bytes") {
			t.Fatalf("fixture-byte substitution error = %v", err)
		}
	})

	t.Run("expected case", func(t *testing.T) {
		tampered := value
		tampered.Expected.CaseDigest = digestBytes([]byte("fabricated-development-case"))
		tampered.Digest, err = developmentChallengeDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDevelopmentChallenge(tampered); err == nil {
			t.Fatal("stress development challenge accepted an expected-case substitution")
		}
	})

	encoded, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("unknown property", func(t *testing.T) {
		var document map[string]any
		if err := json.Unmarshal(encoded, &document); err != nil {
			t.Fatal(err)
		}
		document["unknown"] = true
		unknown, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeDevelopmentChallenge(bytes.NewReader(unknown)); err == nil {
			t.Fatal("stress development challenge accepted an unknown property")
		}
	})
	t.Run("trailing document", func(t *testing.T) {
		if _, err := DecodeDevelopmentChallenge(bytes.NewReader(append(slices.Clone(encoded), []byte("\n{}")...))); err == nil {
			t.Fatal("stress development challenge accepted trailing JSON")
		}
	})

	t.Run("foreign receipt", func(t *testing.T) {
		receipt, err := VerifyDevelopmentChallenge(value)
		if err != nil {
			t.Fatal(err)
		}
		receipt.ChallengeDigest = digestBytes([]byte("foreign-development-challenge"))
		receipt.Digest, err = developmentChallengeReceiptDigest(receipt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyDevelopmentChallengeReceipt(value, receipt); err == nil {
			t.Fatal("stress development challenge accepted a receipt from another challenge")
		}
	})
}

func TestDevelopmentChallengeIgnoresProviderAndNetworkConfiguration(t *testing.T) {
	root := filepath.Join("..", "..")
	want, err := BuildDevelopmentChallenge(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_API_KEY", "must-not-be-read")
	t.Setenv("DEEPSEEK_API_KEY", "must-not-be-read")
	t.Setenv("EVALWITNESS_BASE_URL", "http://127.0.0.1:1")
	got, err := BuildDevelopmentChallenge(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("provider or network configuration changed the provider-free development challenge")
	}
	if _, err := VerifyDevelopmentChallenge(got); err != nil {
		t.Fatal(err)
	}
}
