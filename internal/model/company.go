package model

// Company lives in companies/<id>.yaml and is shared by all applications to
// that company.
type Company struct {
	SchemaVersion int    `yaml:"schemaVersion"`
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Industry      string `yaml:"industry,omitempty"`
	Size          string `yaml:"size,omitempty"`
	HQ            string `yaml:"hq,omitempty"`
	Website       string `yaml:"website,omitempty"`
	Notes         string `yaml:"notes,omitempty"`
}
