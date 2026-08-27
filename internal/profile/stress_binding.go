package profile

// StressBinding links TASK 056 stress to 068 admitted relations.
type StressBinding struct {
	Task   string `json:"task"`
	Status Status `json:"status"`
	Source string `json:"source"`
}

// StressBindings returns stress bindings.
func StressBindings() []StressBinding {
	return []StressBinding{
		{Task: "056", Status: StatusMeasured, Source: "068 admitted"},
	}
}
