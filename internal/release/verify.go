package release

import (
	"errors"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func Verify(assetRoot string, manifestRaw, sbomRaw, statementRaw, envelopeRaw, trustRootRaw, policyRaw []byte, allowUnsigned bool) (VerificationReport, error) {
	manifest, err := DecodeManifest(manifestRaw)
	if err != nil {
		return VerificationReport{}, err
	}
	if err := VerifyManifestAssets(assetRoot, manifest); err != nil {
		return VerificationReport{}, err
	}
	if _, err := VerifySBOM(assetRoot, manifest, sbomRaw); err != nil {
		return VerificationReport{}, err
	}
	if _, err := DecodeStatement(statementRaw, manifestRaw, sbomRaw); err != nil {
		return VerificationReport{}, err
	}
	signingInputs := 0
	for _, raw := range [][]byte{envelopeRaw, trustRootRaw, policyRaw} {
		if len(raw) > 0 {
			signingInputs++
		}
	}
	if signingInputs != 0 && signingInputs != 3 {
		return VerificationReport{}, errors.New("release signature verification requires envelope, trust root, and policy together")
	}
	verifiedKeyIDs := []string{}
	if signingInputs == 3 {
		verifiedKeyIDs, err = VerifySignedStatement(manifestRaw, sbomRaw, statementRaw, envelopeRaw, trustRootRaw, policyRaw)
		if err != nil {
			return VerificationReport{}, err
		}
	} else if !allowUnsigned {
		return VerificationReport{}, errors.New("release is unsigned; pass an explicit unsigned-development allowance or supply signature material")
	}
	return VerificationReport{
		SchemaVersion: VerificationSchemaVersion, Product: manifest.Product, ProductVersion: manifest.ProductVersion,
		GitCommit: manifest.GitCommit, ManifestDigest: protocol.DigestBytes(manifestRaw), SBOMDigest: protocol.DigestBytes(sbomRaw),
		StatementDigest: protocol.DigestBytes(statementRaw), AssetCount: manifest.AssetCount, TotalBytes: manifest.TotalBytes,
		Signed: signingInputs == 3, VerifiedKeyIDs: verifiedKeyIDs, Valid: true,
	}, nil
}
