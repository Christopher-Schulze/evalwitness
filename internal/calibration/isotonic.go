package calibration

import "sort"

// IsotonicBlock is one PAVA block with mean of y.
type IsotonicBlock struct {
	Xs []float64 `json:"xs"`
	Y  float64   `json:"y"`
}

// IsotonicModel is a monotone step function from sorted predicted to calibrated.
type IsotonicModel struct {
	Blocks []IsotonicBlock `json:"blocks"`
}

// FitIsotonic fits isotonic regression via PAVA. Observations must be calibration only.
// Predicted is the raw score, Won is label.
func FitIsotonic(observations []Observation) IsotonicModel {
	type pair struct {
		x float64
		y float64
	}
	var pairs []pair
	for _, o := range observations {
		if o.Won == nil {
			continue
		}
		y := 0.0
		if *o.Won {
			y = 1
		}
		pairs = append(pairs, pair{x: o.Predicted, y: y})
	}
	if len(pairs) == 0 {
		return IsotonicModel{}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].x < pairs[j].x })
	// PAVA
	var blocks []IsotonicBlock
	for _, p := range pairs {
		blocks = append(blocks, IsotonicBlock{Xs: []float64{p.x}, Y: p.y})
		// merge while monotone violated
		for len(blocks) >= 2 && blocks[len(blocks)-2].Y > blocks[len(blocks)-1].Y {
			last := blocks[len(blocks)-1]
			prev := blocks[len(blocks)-2]
			mergedXs := append(append([]float64(nil), prev.Xs...), last.Xs...)
			mergedY := (prev.Y*float64(len(prev.Xs)) + last.Y*float64(len(last.Xs))) / float64(len(mergedXs))
			blocks = blocks[:len(blocks)-2]
			blocks = append(blocks, IsotonicBlock{Xs: mergedXs, Y: mergedY})
		}
	}
	return IsotonicModel{Blocks: blocks}
}

// Calibrate maps predicted to isotonic calibrated value (piecewise constant).
func (m IsotonicModel) Calibrate(predicted float64) float64 {
	if len(m.Blocks) == 0 {
		return predicted
	}
	// find block containing predicted or nearest
	for _, b := range m.Blocks {
		if len(b.Xs) == 0 {
			continue
		}
		// blocks are sorted by x, Xs within block are sorted
		minX := b.Xs[0]
		maxX := b.Xs[len(b.Xs)-1]
		if predicted >= minX && predicted <= maxX {
			return b.Y
		}
		if predicted < minX {
			return b.Y
		}
	}
	// beyond max -> last block
	return m.Blocks[len(m.Blocks)-1].Y
}
