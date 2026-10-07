package model

import (
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// DefaultConfigYAML is written to data/config.yaml on first run.
const DefaultConfigYAML = `schemaVersion: 1
defaultCurrency: EUR          # change to your currency
staleAfterDays: 14            # active applications with no contact for this long are flagged
upcomingDays: 14              # dashboard horizon
priorities:                   # id is stored in application files; label/color are display-only
  - { id: high,   label: High,   color: red }
  - { id: medium, label: Medium, color: amber }
  - { id: low,    label: Low,    color: grey }
statuses:                     # order = kanban column order; closed = hidden from "active" views
  - { id: wishlist,     label: Wishlist }
  - { id: applied,      label: Applied }
  - { id: screening,    label: Screening }
  - { id: interviewing, label: Interviewing }
  - { id: offer,        label: Offer }
  - { id: accepted,     label: Accepted,    closed: true }
  - { id: rejected,     label: Rejected,    closed: true }
  - { id: withdrawn,    label: Withdrawn,   closed: true }
  - { id: ghosted,      label: No response, closed: true }
channels:       [Company website, LinkedIn, Referral, Recruiter (inbound), Recruitment agency, Job board, Other]
contractTypes:  [Permanent, Fixed-term, Contractor, Freelance, Part-time, Internship]
decisions:      [Pending, Offer accepted, Offer declined, Rejected by company, Withdrew]
companySizes:   ["1-10", "11-50", "51-200", "201-1000", "1001-5000", "5000+"]
interviewTypes: [Phone screen, Technical, System design, Behavioural, Case study, Panel, Final, Other]
salaryPeriods:  [year, month, day, hour]
`

// Config holds the dropdown lists and settings from config.yaml.
type Config struct {
	SchemaVersion   int        `yaml:"schemaVersion"`
	DefaultCurrency string     `yaml:"defaultCurrency"`
	StaleAfterDays  int        `yaml:"staleAfterDays"`
	UpcomingDays    int        `yaml:"upcomingDays"`
	Priorities      []Priority `yaml:"priorities"`
	Statuses        []Status   `yaml:"statuses"`
	Channels        []string   `yaml:"channels"`
	ContractTypes   []string   `yaml:"contractTypes"`
	Decisions       []string   `yaml:"decisions"`
	CompanySizes    []string   `yaml:"companySizes"`
	InterviewTypes  []string   `yaml:"interviewTypes"`
	SalaryPeriods   []string   `yaml:"salaryPeriods"`
}

type Priority struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
	Color string `yaml:"color"`
}

type Status struct {
	ID     string `yaml:"id"`
	Label  string `yaml:"label"`
	Closed bool   `yaml:"closed,omitempty"`
}

// DefaultConfig returns the built-in defaults.
func DefaultConfig() Config {
	var c Config
	if err := yaml.Unmarshal([]byte(DefaultConfigYAML), &c); err != nil {
		panic("model: bad default config: " + err.Error())
	}
	return c
}

// ParseConfig decodes config.yaml and fills anything missing from the defaults.
func ParseConfig(data []byte) (Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return DefaultConfig(), err
	}
	def := DefaultConfig()
	if c.SchemaVersion == 0 {
		c.SchemaVersion = SchemaVersion
	}
	if c.DefaultCurrency == "" {
		c.DefaultCurrency = def.DefaultCurrency
	}
	if c.StaleAfterDays <= 0 {
		c.StaleAfterDays = def.StaleAfterDays
	}
	if c.UpcomingDays <= 0 {
		c.UpcomingDays = def.UpcomingDays
	}
	c.Priorities = slices.DeleteFunc(c.Priorities, func(p Priority) bool { return p.ID == "" })
	c.Statuses = slices.DeleteFunc(c.Statuses, func(s Status) bool { return s.ID == "" })
	for i := range c.Priorities {
		if c.Priorities[i].Label == "" {
			c.Priorities[i].Label = c.Priorities[i].ID
		}
	}
	for i := range c.Statuses {
		if c.Statuses[i].Label == "" {
			c.Statuses[i].Label = c.Statuses[i].ID
		}
	}
	fill := func(dst *[]string, src []string) {
		if len(*dst) == 0 {
			*dst = src
		}
	}
	if len(c.Priorities) == 0 {
		c.Priorities = def.Priorities
	}
	if len(c.Statuses) == 0 {
		c.Statuses = def.Statuses
	}
	fill(&c.Channels, def.Channels)
	fill(&c.ContractTypes, def.ContractTypes)
	fill(&c.Decisions, def.Decisions)
	fill(&c.CompanySizes, def.CompanySizes)
	fill(&c.InterviewTypes, def.InterviewTypes)
	fill(&c.SalaryPeriods, def.SalaryPeriods)
	return c, nil
}

// Status returns the status with the given id.
func (c *Config) Status(id string) (Status, bool) {
	i := slices.IndexFunc(c.Statuses, func(s Status) bool { return s.ID == id })
	if i < 0 {
		return Status{ID: id, Label: id}, false
	}
	return c.Statuses[i], true
}

// Priority returns the priority with the given id.
func (c *Config) Priority(id string) (Priority, bool) {
	i := slices.IndexFunc(c.Priorities, func(p Priority) bool { return p.ID == id })
	if i < 0 {
		return Priority{ID: id, Label: id}, false
	}
	return c.Priorities[i], true
}

// StatusLabel returns the display label, or the raw id if it is unknown.
func (c *Config) StatusLabel(id string) string { s, _ := c.Status(id); return s.Label }

// PriorityLabel returns the display label, or the raw id if it is unknown.
func (c *Config) PriorityLabel(id string) string { p, _ := c.Priority(id); return p.Label }

// IsActive reports whether an application with this status is still open.
// Unknown statuses count as active so they stay visible.
func (c *Config) IsActive(status string) bool {
	s, _ := c.Status(status)
	return !s.Closed
}

// StatusIndex returns the position of the status in the configured order,
// or len(statuses) if it is unknown.
func (c *Config) StatusIndex(id string) int {
	if i := slices.IndexFunc(c.Statuses, func(s Status) bool { return s.ID == id }); i >= 0 {
		return i
	}
	return len(c.Statuses)
}

// PriorityIndex returns the position of the priority in the configured
// order, or len(priorities) if it is unknown or unset.
func (c *Config) PriorityIndex(id string) int {
	if i := slices.IndexFunc(c.Priorities, func(p Priority) bool { return p.ID == id }); i >= 0 {
		return i
	}
	return len(c.Priorities)
}

// DefaultStatus is used for new applications.
func (c *Config) DefaultStatus() string {
	if _, ok := c.Status("wishlist"); ok {
		return "wishlist"
	}
	return c.Statuses[0].ID
}

// DefaultPriority is used for new applications.
func (c *Config) DefaultPriority() string {
	if _, ok := c.Priority("medium"); ok {
		return "medium"
	}
	return c.Priorities[len(c.Priorities)/2].ID
}

// decisionStatus maps the default decision labels to the status they imply.
var decisionStatus = map[string]string{
	"offer accepted":      "accepted",
	"offer declined":      "withdrawn",
	"rejected by company": "rejected",
	"withdrew":            "withdrawn",
}

// SuggestedStatus returns the status that matches a decision, if any. The
// app offers it as a one-click change but never applies it automatically.
func (c *Config) SuggestedStatus(decision string) (Status, bool) {
	id, ok := decisionStatus[strings.ToLower(strings.TrimSpace(decision))]
	if !ok {
		return Status{}, false
	}
	return c.Status(id)
}

// Has reports whether value is one of the options (exact match).
func Has(options []string, value string) bool { return slices.Contains(options, value) }
