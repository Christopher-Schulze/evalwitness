package profile

// ExplorerPath is the interactive rendering path for profile.
type ExplorerPath struct {
	ProfileID string `json:"profile_id"`
	Path      string `json:"path"`
}

// ExplorerPaths returns explorer rendering paths.
func ExplorerPaths() []ExplorerPath {
	return []ExplorerPath{
		{ProfileID: "default", Path: "/explorer/profile"},
	}
}
