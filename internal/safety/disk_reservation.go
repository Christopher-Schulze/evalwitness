package safety

import (
	"errors"
	"os"
	"path/filepath"
)

const reservationChunkBytes = 1024 * 1024

type diskReservation struct {
	file      *os.File
	path      string
	remaining int64
}

func newDiskReservation(directory string, bytes int64) (*diskReservation, error) {
	if bytes < 0 {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationCreate, Path: directory}
	}
	file, err := os.OpenFile(filepath.Join(directory, ".evalwitness-reservation"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, SensitiveFileMode)
	if err != nil {
		return nil, &Error{Kind: ErrorInsufficientSpace, Operation: OperationCreate, Path: directory, Cause: err}
	}
	reservation := &diskReservation{file: file, path: file.Name(), remaining: bytes}
	if err := reservation.allocate(bytes); err != nil {
		_ = reservation.close()
		return nil, err
	}
	return reservation, nil
}

func (r *diskReservation) allocate(bytes int64) error {
	chunk := make([]byte, reservationChunkBytes)
	for written := int64(0); written < bytes; {
		remaining := bytes - written
		if remaining < int64(len(chunk)) {
			chunk = chunk[:remaining]
		}
		count, err := r.file.Write(chunk)
		if err != nil {
			return &Error{Kind: ErrorInsufficientSpace, Operation: OperationCreate, Path: filepath.Dir(r.path), Cause: err}
		}
		if count != len(chunk) {
			return &Error{Kind: ErrorInsufficientSpace, Operation: OperationCreate, Path: filepath.Dir(r.path)}
		}
		written += int64(count)
	}
	if err := r.file.Sync(); err != nil {
		return &Error{Kind: ErrorInsufficientSpace, Operation: OperationCreate, Path: filepath.Dir(r.path), Cause: err}
	}
	return nil
}

func (r *diskReservation) release(bytes int64) error {
	if bytes < 0 || bytes > r.remaining {
		return &Error{Kind: ErrorResourceLimit, Operation: OperationExtract, Path: filepath.Dir(r.path)}
	}
	r.remaining -= bytes
	if err := r.file.Truncate(r.remaining); err != nil {
		return &Error{Kind: ErrorInsufficientSpace, Operation: OperationExtract, Path: filepath.Dir(r.path), Cause: err}
	}
	return nil
}

func (r *diskReservation) close() error {
	if r == nil || r.file == nil {
		return nil
	}
	closeErr := r.file.Close()
	removeErr := os.Remove(r.path)
	r.file = nil
	if closeErr != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Path: filepath.Dir(r.path), Cause: closeErr}
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationExtract, Path: filepath.Dir(r.path), Cause: removeErr}
	}
	return nil
}
