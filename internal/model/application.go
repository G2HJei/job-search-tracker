package model

import (
	"slices"
	"strings"
	"time"
)

// SchemaVersion is written to every data file.
const SchemaVersion = 1

// Application is one job application. Everything except the Company group
// lives in applications/<id>/application.yaml.
type Application struct {
	SchemaVersion int       `yaml:"schemaVersion"`
	ID            string    `yaml:"id"`
	CreatedAt     time.Time `yaml:"createdAt"`
	UpdatedAt     time.Time `yaml:"updatedAt"`

	Priority  string `yaml:"priority"`  // config.priorities[].id
	Status    string `yaml:"status"`    // config.statuses[].id
	CompanyID string `yaml:"companyId"` // companies/<id>.yaml

	Job          Job          `yaml:"job"`
	Contacts     Contacts     `yaml:"contacts,omitempty"`
	Process      Process      `yaml:"process,omitempty"`
	Compensation Compensation `yaml:"compensation,omitempty"`
	Other        Other        `yaml:"other,omitempty"`
}

type Job struct {
	Title            string      `yaml:"title"`
	PostingURL       string      `yaml:"postingUrl,omitempty"`
	PostingCopy      string      `yaml:"postingCopy,omitempty"`
	JobID            string      `yaml:"jobId,omitempty"`
	Department       string      `yaml:"department,omitempty"`
	Location         string      `yaml:"location,omitempty"`
	KeyRequirements  string      `yaml:"keyRequirements,omitempty"`
	Fit              int         `yaml:"fit,omitempty"`
	Interest         int         `yaml:"interest,omitempty"`
	Channel          string      `yaml:"channel,omitempty"`
	CVVersion        string      `yaml:"cvVersion,omitempty"`
	ExtraMaterials   []string    `yaml:"extraMaterials,omitempty"`
	AdvertisedSalary SalaryRange `yaml:"advertisedSalary,omitempty,flow"`
}

type Contacts struct {
	Referrer      *Contact  `yaml:"referrer,omitempty"`
	Recruiters    []Contact `yaml:"recruiters,omitempty"`
	HiringManager *Contact  `yaml:"hiringManager,omitempty"`
	Interviewers  []Contact `yaml:"interviewers,omitempty"`
}

type Process struct {
	NextStep              string      `yaml:"nextStep,omitempty"`
	NextStepDate          Date        `yaml:"nextStepDate,omitempty"`
	LastContactDate       Date        `yaml:"lastContactDate,omitempty"`
	DateApplied           Date        `yaml:"dateApplied,omitempty"`
	ScreeningDate         DateTime    `yaml:"screeningDate,omitempty"`
	Interviews            []Interview `yaml:"interviews,omitempty"`
	FollowUpDate          Date        `yaml:"followUpDate,omitempty"`
	OfferDate             Date        `yaml:"offerDate,omitempty"`
	QA                    []QA        `yaml:"qa,omitempty"`
	TakeHome              TakeHome    `yaml:"takeHome,omitempty"`
	Decision              string      `yaml:"decision,omitempty"`
	RejectionFeedback     string      `yaml:"rejectionFeedback,omitempty"`
	OfferResponseDeadline Date        `yaml:"offerResponseDeadline,omitempty"`
}

type Interview struct {
	Date         DateTime `yaml:"date,omitempty"`
	Type         string   `yaml:"type,omitempty"`
	Interviewers []string `yaml:"interviewers,omitempty,flow"` // names from contacts.interviewers
	Notes        string   `yaml:"notes,omitempty"`
}

type QA struct {
	Round    int    `yaml:"round"` // 0 = general/screening, n = interview n
	Question string `yaml:"question"`
	Answer   string `yaml:"answer,omitempty"`
}

type TakeHome struct {
	Details  string   `yaml:"details,omitempty"`
	Deadline DateTime `yaml:"deadline,omitempty"`
}

type Compensation struct {
	Asked        SalaryRange `yaml:"asked,omitempty,flow"`
	Offered      SalaryRange `yaml:"offered,omitempty,flow"`
	ContractType string      `yaml:"contractType,omitempty"`
	Bonus        string      `yaml:"bonus,omitempty"`
	Equity       string      `yaml:"equity,omitempty"`
	Benefits     string      `yaml:"benefits,omitempty"`
	NoticePeriod string      `yaml:"noticePeriod,omitempty"`
	StartDate    Date        `yaml:"startDate,omitempty"`
}

type Other struct {
	LessonsLearned string `yaml:"lessonsLearned,omitempty"`
	Notes          string `yaml:"notes,omitempty"`
}

// Clone returns a deep copy, so callers can modify it without touching the
// store's copy.
func (a Application) Clone() Application {
	c := a
	c.Job.ExtraMaterials = slices.Clone(a.Job.ExtraMaterials)
	c.Contacts.Referrer = a.Contacts.Referrer.clone()
	c.Contacts.HiringManager = a.Contacts.HiringManager.clone()
	c.Contacts.Recruiters = slices.Clone(a.Contacts.Recruiters)
	c.Contacts.Interviewers = slices.Clone(a.Contacts.Interviewers)
	c.Process.Interviews = slices.Clone(a.Process.Interviews)
	for i := range c.Process.Interviews {
		c.Process.Interviews[i].Interviewers = slices.Clone(a.Process.Interviews[i].Interviewers)
	}
	c.Process.QA = slices.Clone(a.Process.QA)
	return c
}

// LatestContact returns the later of the last contact date and the date
// applied, or the zero Date if neither is set.
func (a *Application) LatestContact() Date {
	d := a.Process.LastContactDate
	if a.Process.DateApplied.After(d) {
		d = a.Process.DateApplied
	}
	return d
}

// ApplyStatus sets the status. When the status first moves to "applied" and
// no application date is recorded, the date applied is set to today.
func (a *Application) ApplyStatus(status string, today Date) {
	a.Status = status
	if status == "applied" && a.Process.DateApplied.IsZero() {
		a.Process.DateApplied = today
	}
}

// ContactNames returns the names of every contact on the application.
func (a *Application) ContactNames() []string {
	var names []string
	for _, c := range a.Contacts.All() {
		names = append(names, c.Name)
	}
	return names
}

// SearchText returns lower-cased text used for free-text search.
func (a *Application) SearchText(companyName string) string {
	parts := []string{
		companyName, a.CompanyID, a.Job.Title, a.Job.Location, a.Job.Department,
		a.Job.JobID, a.Job.Channel, a.Process.NextStep, a.Other.Notes, a.Other.LessonsLearned,
	}
	for _, c := range a.Contacts.All() {
		parts = append(parts, c.Name, c.Role, c.Email, c.Notes)
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

// ShiftDates moves every date on the application by n days. Used to keep the
// demo data current.
func (a *Application) ShiftDates(n int) {
	shift := func(d *Date) {
		if !d.IsZero() {
			*d = d.AddDays(n)
		}
	}
	a.CreatedAt = a.CreatedAt.AddDate(0, 0, n)
	a.UpdatedAt = a.UpdatedAt.AddDate(0, 0, n)
	p := &a.Process
	shift(&p.NextStepDate)
	shift(&p.LastContactDate)
	shift(&p.DateApplied)
	shift(&p.FollowUpDate)
	shift(&p.OfferDate)
	shift(&p.OfferResponseDeadline)
	p.ScreeningDate = p.ScreeningDate.AddDays(n)
	p.TakeHome.Deadline = p.TakeHome.Deadline.AddDays(n)
	for i := range p.Interviews {
		p.Interviews[i].Date = p.Interviews[i].Date.AddDays(n)
	}
	shift(&a.Compensation.StartDate)
}
