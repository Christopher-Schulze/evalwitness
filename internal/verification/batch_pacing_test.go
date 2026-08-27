package verification

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitBatchDispatchIntervalCompletesAndCancels(t *testing.T) {
	started := time.Now()
	if err := waitBatchDispatchInterval(context.Background(), 5*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 5*time.Millisecond {
		t.Fatal("batch dispatch interval returned before its locked duration")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitBatchDispatchInterval(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled pacing wait = %v, want context.Canceled", err)
	}
}
