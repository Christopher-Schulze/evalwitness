package main

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestRequireJSONEOFRejectsSecondValue(t *testing.T) {
	decoder := json.NewDecoder(bytes.NewBufferString(`{} {}`))
	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if err := requireJSONEOF(decoder); err == nil {
		t.Fatal("second JSON value was accepted")
	}
}

func TestRequireJSONEOFAcceptsWhitespace(t *testing.T) {
	decoder := json.NewDecoder(bytes.NewBufferString("{}  \n"))
	var first json.RawMessage
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if err := requireJSONEOF(decoder); err != nil && err != io.EOF {
		t.Fatal(err)
	}
}
