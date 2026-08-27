package main

import (
	"errors"
	"io"
	"testing"
)

type failingOutputWriter struct {
	written int
	err     error
}

func (writer failingOutputWriter) Write(_ []byte) (int, error) {
	return writer.written, writer.err
}

func TestWriteOutput(t *testing.T) {
	t.Parallel()

	writeFailure := errors.New("disk full")
	tests := []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{name: "complete", writer: io.Discard},
		{name: "write error", writer: failingOutputWriter{err: writeFailure}, want: writeFailure},
		{name: "short write", writer: failingOutputWriter{written: 1}, want: io.ErrShortWrite},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := writeOutput(test.writer, []byte("{}\n"))
			if test.want == nil && err != nil {
				t.Fatalf("writeOutput() error = %v, want nil", err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("writeOutput() error = %v, want %v", err, test.want)
			}
		})
	}
}
