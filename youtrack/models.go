package youtrack

// Issue represents a YouTrack issue.
type Issue struct {
	ID           string        `json:"id,omitempty"`
	IDReadable   string        `json:"idReadable,omitempty"`
	Summary      string        `json:"summary,omitempty"`
	Description  string        `json:"description,omitempty"`
	Updated      int64         `json:"updated,omitempty"`
	Project      *Project      `json:"project,omitempty"`
	CustomFields []CustomField `json:"customFields,omitempty"`
	// Add other fields as needed for synchronization
}

// Project represents a YouTrack project.
type Project struct {
	YouTrackType
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"shortName,omitempty"`
}

// CustomField represents a custom field in YouTrack.
type CustomField struct {
	YouTrackType
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Value interface{} `json:"value,omitempty"` // Value can be string, int, object, etc.
}

// DateCustomField represents a custom field of type date.
type DateCustomField struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Value int64  `json:"value,omitempty"` // Unix timestamp in milliseconds
}

// StateBundleElement represents a state value in a state custom field.
type StateBundleElement struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// StateCustomField represents a custom field of type state.
type StateCustomField struct {
	ID    string              `json:"id,omitempty"`
	Name  string              `json:"name,omitempty"`
	Value *StateBundleElement `json:"value,omitempty"`
}

// YouTrack API responses often include a "$type" field.
type YouTrackType struct {
	Type string `json:"$type"`
}

// IssueWrapper is used for creating issues with a specific $type
type IssueWrapper struct {
	YouTrackType
	Summary      string        `json:"summary"`
	Description  string        `json:"description"`
	Project      *Project      `json:"project"`
	CustomFields []CustomField `json:"customFields,omitempty"`
}

// CustomFieldWrapper is used for updating custom fields with a specific $type
type CustomFieldWrapper struct {
	YouTrackType
	Value interface{} `json:"value"`
}
