package youtrack

type Issue struct {
	ID         string   `json:"id,omitempty"`
	IDReadable string   `json:"idReadable,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Updated    int64    `json:"updated,omitempty"`
	Project    *Project `json:"project,omitempty"`
}

type Project struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"shortName,omitempty"`
}
