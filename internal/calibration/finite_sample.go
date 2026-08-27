package calibration

import (
	"fmt"
	"math"
)

const (
	FiniteSampleEvaluated   = "evaluated"
	FiniteSampleUnsupported = "unsupported"
	FiniteSampleUnitTask    = "unique_task"
)

type FiniteSampleControl struct {
	Status              string   `json:"status"`
	Reason              string   `json:"reason,omitempty"`
	ExchangeabilityUnit string   `json:"exchangeability_unit"`
	UniqueTasks         int      `json:"unique_tasks"`
	RepeatedTasks       int      `json:"repeated_tasks"`
	Selected            int      `json:"selected"`
	UpperBound          *float64 `json:"upper_bound,omitempty"`
}

func EvaluateFiniteSample(observations []Observation, threshold float64, estimate func(Observation) float64) FiniteSampleControl {
	control := FiniteSampleControl{ExchangeabilityUnit: FiniteSampleUnitTask}
	counts := map[string]int{}
	for _, observation := range observations {
		if observation.TaskID == "" {
			return FiniteSampleControl{
				Status: FiniteSampleUnsupported, Reason: "task_id missing; exchangeability unit cannot be verified",
				ExchangeabilityUnit: FiniteSampleUnitTask,
			}
		}
		counts[observation.TaskID]++
	}
	control.UniqueTasks = len(counts)
	for _, n := range counts {
		if n > 1 {
			control.RepeatedTasks++
		}
	}
	if control.RepeatedTasks > 0 {
		control.Status = FiniteSampleUnsupported
		control.Reason = fmt.Sprintf("clustered task dependence: %d tasks have repeated rows; Hoeffding/binomial units are not exchangeable", control.RepeatedTasks)
		return control
	}
	if control.UniqueTasks < 2 {
		control.Status = FiniteSampleUnsupported
		control.Reason = "fewer than two unique tasks; finite-sample bound is unsupported"
		return control
	}
	selected := 0
	errors := 0
	for _, observation := range observations {
		if observation.Won == nil || estimate(observation) < threshold {
			continue
		}
		selected++
		if !*observation.Won {
			errors++
		}
	}
	control.Selected = selected
	if selected == 0 {
		control.Status = FiniteSampleUnsupported
		control.Reason = "no selected decisions; finite-sample risk is undefined"
		return control
	}
	bound := hoeffdingUpperBound(float64(errors)/float64(selected), selected)
	control.Status = FiniteSampleEvaluated
	control.UpperBound = &bound
	return control
}

func hoeffdingUpperBound(mean float64, n int) float64 {
	if n <= 0 {
		return 1
	}
	width := math.Sqrt(math.Log(2/0.05) / (2 * float64(n)))
	bound := mean + width
	if bound > 1 {
		return 1
	}
	if bound < 0 {
		return 0
	}
	return bound
}
