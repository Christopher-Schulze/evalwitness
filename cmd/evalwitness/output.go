package main

import (
	"fmt"
	"io"
	"os"
)

func writeCommandOutput(scope string, output []byte) int {
	if err := writeOutput(os.Stdout, output); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write output: %v\n", scope, err)
		return 1
	}
	return 0
}

func writeOutput(writer io.Writer, output []byte) error {
	written, err := writer.Write(output)
	if err != nil {
		return err
	}
	if written != len(output) {
		return io.ErrShortWrite
	}
	return nil
}
