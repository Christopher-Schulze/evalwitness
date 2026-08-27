package provider

import _ "embed"

//go:embed testdata/request-fingerprint-v2.json
var requestFingerprintVectorCorpus []byte

func RequestFingerprintVectorCorpus() []byte {
	return append([]byte(nil), requestFingerprintVectorCorpus...)
}
