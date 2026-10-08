// Package sections renders the six field-group cards on the application
// detail page. Each group has a view and an edit partial; the edit partial
// swaps in place of the view card and back again.
package sections

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

// Names lists the sections in display order.
var Names = []string{"company", "job", "contacts", "process", "compensation", "other"}

var titles = map[string]string{
	"company":      "Company",
	"job":          "Job Application",
	"contacts":     "Contacts",
	"process":      "Hiring Process",
	"compensation": "Compensation",
	"other":        "Other",
}

// Valid reports whether name is a section.
func Valid(name string) bool { return slices.Contains(Names, name) }

// Title returns the display title of a section.
func Title(name string) string { return titles[name] }

// FileItem is an attachment or CV file.
type FileItem struct {
	Name string
	Size int64
}

// Data is everything a section card needs.
type Data struct {
	App          model.Application
	Company      model.Company
	CompanyFound bool
	CompanyApps  int // applications sharing this company
	Config       model.Config
	Today        model.Date

	Files      []FileItem // attachments of this application
	CVs        []string   // file names in the CV library
	Companies  []model.Company
	Industries []string

	Errors model.FieldErrors // field errors for an edit partial
	Error  string            // form-level message, e.g. a conflict
	Rev    string            // revision the edit form was rendered from; empty = current
	OOB    bool              // render the view card as an out-of-band swap
}

// Rev returns the revision of one section's data. Edit forms carry it, so a
// save can tell whether that section changed elsewhere in the meantime.
func Rev(name string, a model.Application, c model.Company) string {
	switch name {
	case "company":
		return model.Rev(c)
	case "job":
		return model.Rev(a.Job)
	case "contacts":
		return model.Rev(a.Contacts)
	case "process":
		return model.Rev(a.Process)
	case "compensation":
		return model.Rev(a.Compensation)
	case "other":
		return model.Rev(a.Other)
	}
	return ""
}

func (d Data) rev(name string) string {
	if d.Rev != "" {
		return d.Rev
	}
	return Rev(name, d.App, d.Company)
}

func (d Data) err(key string) string { return d.Errors.Get(key) }

func (d Data) appURL() string { return "/applications/" + d.App.ID }

func (d Data) sectionURL(name string) string { return d.appURL() + "/sections/" + name }

func sectionID(name string) string { return "section-" + name }

func target(name string) string { return "#" + sectionID(name) }

// FileURL links to an attachment of the application.
func (d Data) FileURL(name string) string { return d.appURL() + "/files/" + name }

// HasFile reports whether name is one of the application's attachments.
func (d Data) HasFile(name string) bool {
	return slices.ContainsFunc(d.Files, func(f FileItem) bool { return f.Name == name })
}

func (d Data) fileNames() []string {
	out := make([]string, len(d.Files))
	for i, f := range d.Files {
		out[i] = f.Name
	}
	return out
}

// interviewerNames lists names to suggest for interview rounds.
func (d Data) interviewerNames() []string {
	var out []string
	for _, c := range d.App.Contacts.Interviewers {
		out = append(out, c.Name)
	}
	if hm := d.App.Contacts.HiringManager; hm != nil && hm.Name != "" {
		out = append(out, hm.Name)
	}
	return out
}

// roundOptions are the choices for linking a Q&A entry to a round.
func (d Data) roundOptions(current int) []RoundOption {
	return Rounds(max(len(d.App.Process.Interviews), current, 3))
}

// Rounds returns "General / screening" plus Interview 1…n.
func Rounds(n int) []RoundOption {
	out := []RoundOption{{0, "General / screening"}}
	for i := 1; i <= n; i++ {
		out = append(out, RoundOption{i, fmt.Sprintf("Interview %d", i)})
	}
	return out
}

// RoundOption is one choice in a Q&A round select.
type RoundOption struct {
	N     int
	Label string
}

// field is one row in a view card. Name is the edit form input that
// clicking the row focuses.
type field struct {
	Label string
	Name  string
	Value templ.Component
}

// fields collects non-empty rows and the empty ones (shown as "Not set").
type fields struct {
	rows  []field
	empty []field
}

func (f *fields) add(label, name string, empty bool, value templ.Component) {
	if empty {
		f.empty = append(f.empty, field{Label: label, Name: name})
		return
	}
	f.rows = append(f.rows, field{label, name, value})
}

func (f *fields) text(label, name, s string) {
	f.add(label, name, s == "", templ.Raw(templ.EscapeString(s)))
}

func itoa(n int) string { return strconv.Itoa(n) }

// QAForRound returns the Q&A entries linked to a round.
func QAForRound(qa []model.QA, round int) []model.QA {
	var out []model.QA
	for _, q := range qa {
		if q.Round == round {
			out = append(out, q)
		}
	}
	return out
}

func roundLabel(n int) string {
	if n == 0 {
		return "General / screening"
	}
	return fmt.Sprintf("Interview %d", n)
}

// qaRounds returns the distinct rounds that have Q&A, in order.
func qaRounds(qa []model.QA) []int {
	var out []int
	for _, q := range qa {
		if !slices.Contains(out, q.Round) {
			out = append(out, q.Round)
		}
	}
	slices.Sort(out)
	return out
}

func (d Data) withOOB() Data {
	d.OOB = true
	return d
}

func (d Data) oobAttrs() templ.Attributes {
	if !d.OOB {
		return nil
	}
	return templ.Attributes{"hx-swap-oob": "true"}
}

// The *OrBlank helpers give an edit form one empty row when a list is
// empty, so it can be filled in without JavaScript.

func rowsOrBlank(s []string) []string {
	if len(s) == 0 {
		return []string{""}
	}
	return s
}

func contactsOrBlank(s []model.Contact) []model.Contact {
	if len(s) == 0 {
		return []model.Contact{{}}
	}
	return s
}

func interviewsOrBlank(s []model.Interview) []model.Interview {
	if len(s) == 0 {
		return []model.Interview{{}}
	}
	return s
}

func qaOrBlank(s []model.QA) []model.QA {
	if len(s) == 0 {
		return []model.QA{{}}
	}
	return s
}

func deref(c *model.Contact) model.Contact {
	if c == nil {
		return model.Contact{}
	}
	return *c
}

func joinNames(names []string) string { return strings.Join(names, ", ") }
