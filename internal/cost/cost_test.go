package cost

import (
	"math"
	"testing"
)

func TestEstimateWithRates(t *testing.T) {
	c := New(0.14, 0.03, 0.50, false)
	got := c.Estimate(1_000_000, 1_000_000, 0)
	if got == nil {
		t.Fatalf("expected non-nil cost")
	}
	want := 0.14 + 0.50
	if math.Abs(*got-want) > 1e-9 {
		t.Errorf("cost = %v, want %v", *got, want)
	}
}

func TestEstimateNoRates(t *testing.T) {
	c := New(0, 0, 0, false)
	if got := c.Estimate(100, 100, 0); got != nil {
		t.Errorf("expected nil for unconfigured rates, got %v", *got)
	}
}

func TestSubscription(t *testing.T) {
	c := New(0, 0, 0, true)
	got := c.Estimate(10_000_000, 10_000_000, 0)
	if got == nil || *got != 0 {
		t.Errorf("expected 0 cost in subscription mode, got %v", got)
	}
}

func TestCachedDiscount(t *testing.T) {
	c := New(0.14, 0.014, 0.50, false)
	full := c.Estimate(1_000_000, 0, 0)
	cached := c.Estimate(1_000_000, 0, 1_000_000)
	if *cached >= *full {
		t.Errorf("cached cost should be less; cached=%v full=%v", *cached, *full)
	}
}

func TestNilCalculator(t *testing.T) {
	var c *Calculator
	if got := c.Estimate(100, 50, 0); got != nil {
		t.Errorf("nil calculator should return nil")
	}
}
