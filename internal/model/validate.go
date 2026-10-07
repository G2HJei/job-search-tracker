package model

import (
	"fmt"
	"net/url"
	"strings"
)

// FieldErrors maps a field key (e.g. "job.title", "contacts.recruiters.1.name")
// to a message.
type FieldErrors map[string]string

// Add records msg for field unless the field already has an error.
func (fe FieldErrors) Add(field, msg string) {
	if _, ok := fe[field]; !ok {
		fe[field] = msg
	}
}

// Get returns the message for field, or "".
func (fe FieldErrors) Get(field string) string { return fe[field] }

// Merge copies all errors from other.
func (fe FieldErrors) Merge(other FieldErrors) {
	for k, v := range other {
		fe.Add(k, v)
	}
}

// ValidURL reports whether s is empty or an absolute http/https URL.
func ValidURL(s string) bool {
	if s == "" {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// IsURL reports whether s looks like a link rather than a file name.
func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

const urlMsg = "Must be an http:// or https:// link"

func checkURL(fe FieldErrors, field, s string) {
	if !ValidURL(s) {
		fe.Add(field, urlMsg)
	}
}

func checkRating(fe FieldErrors, field string, n int) {
	if n < 0 || n > 5 {
		fe.Add(field, "Must be between 1 and 5")
	}
}

func checkSalary(fe FieldErrors, field string, s SalaryRange) {
	switch {
	case s.Min < 0 || s.Max < 0:
		fe.Add(field, "Amounts can't be negative")
	case s.Min > 0 && s.Max > 0 && s.Min > s.Max:
		fe.Add(field, "Minimum is higher than maximum")
	}
}

func checkContact(fe FieldErrors, field string, c *Contact) {
	if c == nil || c.IsZero() {
		return
	}
	if strings.TrimSpace(c.Name) == "" {
		fe.Add(field+".name", "Name is required")
	}
	checkURL(fe, field+".linkedin", c.LinkedIn)
}

// ValidateCompany checks a company record.
func ValidateCompany(c *Company) FieldErrors {
	fe := FieldErrors{}
	if strings.TrimSpace(c.Name) == "" {
		fe.Add("company.name", "Company name is required")
	}
	checkURL(fe, "company.website", c.Website)
	return fe
}

// ValidateJob checks the Job Application group.
func ValidateJob(j *Job) FieldErrors {
	fe := FieldErrors{}
	if strings.TrimSpace(j.Title) == "" {
		fe.Add("job.title", "Job title is required")
	}
	checkURL(fe, "job.postingUrl", j.PostingURL)
	checkRating(fe, "job.fit", j.Fit)
	checkRating(fe, "job.interest", j.Interest)
	checkSalary(fe, "job.advertisedSalary", j.AdvertisedSalary)
	for i, m := range j.ExtraMaterials {
		if u, err := url.Parse(m); err == nil && u.Scheme != "" && !ValidURL(m) {
			fe.Add(fmt.Sprintf("job.extraMaterials.%d", i), "Use a file name or an http(s) link")
		}
	}
	return fe
}

// ValidateContacts checks the Contacts group.
func ValidateContacts(c *Contacts) FieldErrors {
	fe := FieldErrors{}
	checkContact(fe, "contacts.referrer", c.Referrer)
	checkContact(fe, "contacts.hiringManager", c.HiringManager)
	for i := range c.Recruiters {
		checkContact(fe, fmt.Sprintf("contacts.recruiters.%d", i), &c.Recruiters[i])
	}
	for i := range c.Interviewers {
		checkContact(fe, fmt.Sprintf("contacts.interviewers.%d", i), &c.Interviewers[i])
	}
	return fe
}

// ValidateProcess checks the Hiring Process group.
func ValidateProcess(p *Process) FieldErrors {
	fe := FieldErrors{}
	for i, q := range p.QA {
		if strings.TrimSpace(q.Question) == "" {
			fe.Add(fmt.Sprintf("process.qa.%d.question", i), "Question is required")
		}
		if q.Round < 0 {
			fe.Add(fmt.Sprintf("process.qa.%d.round", i), "Round can't be negative")
		}
	}
	return fe
}

// ValidateCompensation checks the Compensation group.
func ValidateCompensation(c *Compensation) FieldErrors {
	fe := FieldErrors{}
	checkSalary(fe, "compensation.asked", c.Asked)
	checkSalary(fe, "compensation.offered", c.Offered)
	return fe
}

// ValidateApplication checks the whole application. Errors block saving;
// warnings (unknown status or priority) are only shown.
func ValidateApplication(a *Application, cfg *Config) (errs, warnings FieldErrors) {
	errs = FieldErrors{}
	warnings = FieldErrors{}
	if strings.TrimSpace(a.CompanyID) == "" {
		errs.Add("companyId", "Company is required")
	}
	errs.Merge(ValidateJob(&a.Job))
	errs.Merge(ValidateContacts(&a.Contacts))
	errs.Merge(ValidateProcess(&a.Process))
	errs.Merge(ValidateCompensation(&a.Compensation))
	if _, ok := cfg.Status(a.Status); !ok {
		warnings.Add("status", fmt.Sprintf("Status %q is not in config.yaml", a.Status))
	}
	if _, ok := cfg.Priority(a.Priority); !ok && a.Priority != "" {
		warnings.Add("priority", fmt.Sprintf("Priority %q is not in config.yaml", a.Priority))
	}
	return errs, warnings
}
