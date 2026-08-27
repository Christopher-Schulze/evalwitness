package profile

// ConstructGeneration tracks historical defect discovery.
type ConstructGeneration struct {
	Version string `json:"version"`
	Defects int    `json:"defects"`
	Status  Status `json:"status"`
}

// Generations returns historical generations.
func Generations() []ConstructGeneration {
	return []ConstructGeneration{
		{Version: "v1", Defects: 3, Status: StatusFailed},
		{Version: "v2", Defects: 5, Status: StatusFailed},
		{Version: "v3", Defects: 0, Status: StatusMeasured},
	}
}
