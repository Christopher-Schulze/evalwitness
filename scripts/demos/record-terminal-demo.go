package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type castHeader struct {
	Version       int               `json:"version"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Title         string            `json:"title"`
	IdleTimeLimit float64           `json:"idle_time_limit"`
	Environment   map[string]string `json:"env"`
}

type recordingWriter struct {
	mu       sync.Mutex
	start    time.Time
	terminal *os.File
	record   *os.File
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "record terminal demo:", err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	flags := flag.NewFlagSet("record-terminal-demo", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	destination := flags.String("destination", "", "new asciicast v2 destination")
	width := flags.Int("width", 120, "terminal columns")
	height := flags.Int("height", 36, "terminal rows")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	command := flags.Args()
	if *destination == "" || len(command) == 0 || *width < 40 || *height < 12 {
		return errors.New("--destination, a command after --, width >= 40, and height >= 12 are required")
	}
	absoluteDestination, err := filepath.Abs(*destination)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absoluteDestination); err == nil {
		return errors.New("destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	record, err := os.CreateTemp(filepath.Dir(absoluteDestination), ".evalwitness-terminal-demo-*.cast")
	if err != nil {
		return err
	}
	temporaryPath := record.Name()
	recordOpen := true
	defer func() {
		if recordOpen {
			runErr = errors.Join(runErr, record.Close())
		}
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			runErr = errors.Join(runErr, removeErr)
		}
	}()
	if err := writeHeader(record, *width, *height); err != nil {
		return err
	}
	writer := &recordingWriter{start: time.Now(), terminal: os.Stdout, record: record}
	process := exec.Command(command[0], command[1:]...)
	process.Stdin = os.Stdin
	process.Stdout = writer
	process.Stderr = writer
	if err := process.Run(); err != nil {
		return fmt.Errorf("recorded command failed: %w", err)
	}
	if err := record.Sync(); err != nil {
		return err
	}
	if err := record.Chmod(0o644); err != nil {
		return err
	}
	if err := record.Close(); err != nil {
		return err
	}
	recordOpen = false
	if err := os.Link(temporaryPath, absoluteDestination); err != nil {
		return fmt.Errorf("publish recording without overwrite: %w", err)
	}
	return nil
}

func writeHeader(record *os.File, width, height int) error {
	header := castHeader{
		Version: 2, Width: width, Height: height,
		Title: "EvalWitness offline claim proof", IdleTimeLimit: 1.5,
		Environment: map[string]string{"SHELL": "/bin/bash", "TERM": "xterm-256color"},
	}
	return json.NewEncoder(record).Encode(header)
}

func (writer *recordingWriter) Write(raw []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if _, err := writer.terminal.Write(raw); err != nil {
		return 0, err
	}
	terminalOutput := normalizeTerminalNewlines(raw)
	encoded, err := json.Marshal(string(terminalOutput))
	if err != nil {
		return 0, err
	}
	if _, err := fmt.Fprintf(writer.record, "[%.6f,\"o\",%s]\n", time.Since(writer.start).Seconds(), encoded); err != nil {
		return 0, err
	}
	return len(raw), nil
}

func normalizeTerminalNewlines(raw []byte) []byte {
	normalized := make([]byte, 0, len(raw)+8)
	for index, value := range raw {
		if value == '\n' && (index == 0 || raw[index-1] != '\r') {
			normalized = append(normalized, '\r')
		}
		normalized = append(normalized, value)
	}
	return normalized
}
