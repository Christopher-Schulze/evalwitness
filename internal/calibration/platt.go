package calibration

import (
	"math"
)

// PlattParams are logistic a + b*score. Deterministic fit via Newton on calibration observations.
type PlattParams struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// FitPlatt fits Platt scaling on calibration observations. Won nil rows are skipped.
// No test leakage: caller must pass only calibration split.
func FitPlatt(observations []Observation) (PlattParams, error) {
	// Collect valid rows.
	var xs, ys []float64
	for _, o := range observations {
		if o.Won == nil {
			continue
		}
		y := 0.0
		if *o.Won {
			y = 1
		}
		// Use conditional diff as primary feature, clamped.
		x := o.ConditionalDiff
		if math.IsNaN(x) || math.IsInf(x, 0) {
			continue
		}
		xs = append(xs, x)
		ys = append(ys, y)
	}
	if len(xs) < 10 {
		return PlattParams{}, nil
	}
	// Newton for logistic: p = 1/(1+exp(-(a + b*x)))
	a, b := 0.0, 1.0
	for iter := 0; iter < 50; iter++ {
		var gA, gB float64
		var hAA, hAB, hBB float64
		for i := range xs {
			z := a + b*xs[i]
			p := 1 / (1 + math.Exp(-z))
			// clip p to avoid log(0)
			if p < 1e-12 {
				p = 1e-12
			}
			if p > 1-1e-12 {
				p = 1 - 1e-12
			}
			err := p - ys[i]
			gA += err
			gB += err * xs[i]
			w := p * (1 - p)
			hAA += w
			hAB += w * xs[i]
			hBB += w * xs[i] * xs[i]
		}
		det := hAA*hBB - hAB*hAB
		if math.Abs(det) < 1e-12 {
			break
		}
		da := (hBB*gA - hAB*gB) / det
		db := (-hAB*gA + hAA*gB) / det
		a -= da
		b -= db
		if math.Abs(da) < 1e-9 && math.Abs(db) < 1e-9 {
			break
		}
	}
	return PlattParams{A: a, B: b}, nil
}

// Calibrate applies Platt scaling.
func (p PlattParams) Calibrate(diff float64) float64 {
	z := p.A + p.B*diff
	return 1 / (1 + math.Exp(-z))
}
