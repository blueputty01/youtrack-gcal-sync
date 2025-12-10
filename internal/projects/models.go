package projects

type Issue struct {
	ID           string        `json:"id,omitempty"`
	IDReadable   string        `json:"idReadable,omitempty"`
	CustomFields []CustomField `json:"customFields,omitempty"`
	Summary      string        `json:"summary,omitempty"`
	Updated      int64         `json:"updated,omitempty"`
	Project      *Project      `json:"project,omitempty"`
}

type CustomField struct {
	YouTrackType
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Value interface{} `json:"value,omitempty"` // Value can be string, int, object, etc.
}

type Project struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"shortName,omitempty"`
}

type YouTrackType struct {
	Type string `json:"$type"`
}
