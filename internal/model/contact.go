package model

import "strings"

type Contact struct {
	Name     string `yaml:"name"`
	Role     string `yaml:"role,omitempty"`
	Email    string `yaml:"email,omitempty"`
	Phone    string `yaml:"phone,omitempty"`
	LinkedIn string `yaml:"linkedin,omitempty"`
	Notes    string `yaml:"notes,omitempty"`
}

// IsZero reports whether every field is blank.
func (c Contact) IsZero() bool {
	return strings.TrimSpace(c.Name+c.Role+c.Email+c.Phone+c.LinkedIn+c.Notes) == ""
}

func (c *Contact) clone() *Contact {
	if c == nil {
		return nil
	}
	v := *c
	return &v
}

// Labelled is a contact together with the part it plays in the application.
type Labelled struct {
	Kind string // Referrer, Recruiter, Hiring manager, Interviewer
	Contact
}

// All returns every contact in display order.
func (cs Contacts) All() []Labelled {
	var out []Labelled
	if cs.Referrer != nil {
		out = append(out, Labelled{"Referrer", *cs.Referrer})
	}
	for _, c := range cs.Recruiters {
		out = append(out, Labelled{"Recruiter", c})
	}
	if cs.HiringManager != nil {
		out = append(out, Labelled{"Hiring manager", *cs.HiringManager})
	}
	for _, c := range cs.Interviewers {
		out = append(out, Labelled{"Interviewer", c})
	}
	return out
}

// IsZero reports whether there are no contacts.
func (cs Contacts) IsZero() bool {
	return cs.Referrer == nil && cs.HiringManager == nil && len(cs.Recruiters) == 0 && len(cs.Interviewers) == 0
}
