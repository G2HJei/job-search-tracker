package web

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/views"
)

func salaryCols(prefix string) []string {
	return []string{prefix + "Min", prefix + "Max", prefix + "Currency", prefix + "Period", prefix + "Note"}
}

func salaryVals(s model.SalaryRange) []string {
	num := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	return []string{num(s.Min), num(s.Max), s.Currency, s.Period, s.Note}
}

func contactString(c *model.Contact) string {
	if c == nil {
		return ""
	}
	parts := []string{c.Name}
	if c.Role != "" {
		parts[0] += " (" + c.Role + ")"
	}
	for _, p := range []string{c.Email, c.Phone, c.LinkedIn} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	s := strings.Join(parts, ", ")
	if c.Notes != "" {
		s += " — " + strings.TrimSpace(c.Notes)
	}
	return s
}

func contactsString(cs []model.Contact) string {
	var out []string
	for i := range cs {
		out = append(out, contactString(&cs[i]))
	}
	return strings.Join(out, "\n")
}

func ratingString(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// exportCSV writes one row per application with every field flattened.
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	header := []string{"id", "createdAt", "updatedAt", "priority", "status",
		"company", "industry", "companySize", "hq", "website",
		"title", "postingUrl", "postingCopy", "jobId", "department", "location", "keyRequirements",
		"fit", "interest", "channel", "cvVersion", "extraMaterials"}
	header = append(header, salaryCols("advertisedSalary")...)
	header = append(header, "referrer", "recruiters", "hiringManager", "interviewers",
		"nextStep", "nextStepDate", "lastContactDate", "dateApplied", "screeningDate", "interviews",
		"followUpDate", "offerDate", "questionsAndAnswers", "takeHomeDetails", "takeHomeDeadline",
		"decision", "rejectionFeedback", "offerResponseDeadline")
	header = append(header, salaryCols("asked")...)
	header = append(header, salaryCols("offered")...)
	header = append(header, "contractType", "bonus", "equity", "benefits", "noticePeriod", "startDate",
		"lessonsLearned", "notes")

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="job-applications-`+s.today().String()+`.csv"`)
	w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM, so Excel reads UTF-8
	cw := csv.NewWriter(w)
	cw.Write(header)
	companies := map[string]model.Company{}
	for _, c := range s.store.ListCompanies() {
		companies[c.ID] = c
	}
	for _, a := range s.store.ListApplications() {
		c := companies[a.CompanyID]
		if c.Name == "" {
			c.Name = a.CompanyID
		}
		j, p, comp := a.Job, a.Process, a.Compensation
		var interviews []string
		for i, iv := range p.Interviews {
			s := fmt.Sprintf("Interview %d: %s %s", i+1, iv.Date.String(), iv.Type)
			if len(iv.Interviewers) > 0 {
				s += " with " + strings.Join(iv.Interviewers, ", ")
			}
			if iv.Notes != "" {
				s += " — " + strings.TrimSpace(iv.Notes)
			}
			interviews = append(interviews, strings.TrimSpace(s))
		}
		var qa []string
		for _, q := range p.QA {
			qa = append(qa, fmt.Sprintf("[round %d] Q: %s\nA: %s", q.Round, q.Question, strings.TrimSpace(q.Answer)))
		}
		row := []string{a.ID, a.CreatedAt.Format(time.RFC3339), a.UpdatedAt.Format(time.RFC3339), a.Priority, a.Status,
			c.Name, c.Industry, c.Size, c.HQ, c.Website,
			j.Title, j.PostingURL, j.PostingCopy, j.JobID, j.Department, j.Location, j.KeyRequirements,
			ratingString(j.Fit), ratingString(j.Interest), j.Channel, j.CVVersion, strings.Join(j.ExtraMaterials, "\n")}
		row = append(row, salaryVals(j.AdvertisedSalary)...)
		row = append(row, contactString(a.Contacts.Referrer), contactsString(a.Contacts.Recruiters),
			contactString(a.Contacts.HiringManager), contactsString(a.Contacts.Interviewers),
			p.NextStep, p.NextStepDate.String(), p.LastContactDate.String(), p.DateApplied.String(),
			p.ScreeningDate.String(), strings.Join(interviews, "\n"),
			p.FollowUpDate.String(), p.OfferDate.String(), strings.Join(qa, "\n\n"), p.TakeHome.Details,
			p.TakeHome.Deadline.String(), p.Decision, p.RejectionFeedback, p.OfferResponseDeadline.String())
		row = append(row, salaryVals(comp.Asked)...)
		row = append(row, salaryVals(comp.Offered)...)
		row = append(row, comp.ContractType, comp.Bonus, comp.Equity, comp.Benefits, comp.NoticePeriod,
			comp.StartDate.String(), a.Other.LessonsLearned, a.Other.Notes)
		cw.Write(row)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		s.log.Error("csv export", "err", err)
	}
}

// icsEscape escapes a TEXT value (RFC 5545 §3.3.11).
func icsEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`).Replace(s)
}

// icsFold writes a content line folded at 75 octets, without splitting a
// UTF-8 character.
func icsFold(b *strings.Builder, line string) {
	for len(line) > 75 {
		cut := 75
		for cut > 0 && line[cut]&0xC0 == 0x80 {
			cut--
		}
		b.WriteString(line[:cut] + "\r\n ")
		line = line[cut:]
	}
	b.WriteString(line + "\r\n")
}

// calendarICS exports every future event on active applications (and start
// dates) as iCalendar. Times are "floating" local times, which is how they
// were entered.
func (s *Server) calendarICS(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	apps := s.store.ListApplications()
	titles := s.titles(apps)
	stamp := s.now().UTC().Format("20060102T150405Z")
	var b strings.Builder
	line := func(l string) { icsFold(&b, l) }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Jedediah//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:Job search")
	for _, e := range model.Future(apps, &cfg, s.today()) {
		uid := e.AppID + "-" + string(e.Kind)
		if e.N > 0 {
			uid += "-" + strconv.Itoa(e.N)
		}
		line("BEGIN:VEVENT")
		line("UID:" + uid + "@jedediah")
		line("DTSTAMP:" + stamp)
		if e.When.HasTime() {
			start := e.When.Time(time.UTC)
			line("DTSTART:" + start.Format("20060102T150405"))
			line("DTEND:" + start.Add(time.Hour).Format("20060102T150405"))
		} else {
			d := e.Date()
			line("DTSTART;VALUE=DATE:" + d.Format("20060102"))
			line("DTEND;VALUE=DATE:" + d.AddDays(1).Format("20060102"))
		}
		line("SUMMARY:" + icsEscape(e.Label+" — "+titles[e.AppID]))
		line("URL:http://" + r.Host + "/applications/" + e.AppID)
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="job-search.ics"`)
	w.Write([]byte(b.String()))
}

func (s *Server) backupZip(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="job-search-backup-`+s.today().String()+`.zip"`)
	if err := s.store.WriteBackup(w); err != nil {
		s.log.Error("backup", "err", err) // headers are already sent
	}
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	d := views.SettingsData{
		Dir:          s.store.Dir(),
		Version:      s.version,
		ConfigYAML:   s.store.ConfigYAML(),
		Warnings:     s.store.Warnings(),
		Applications: len(s.store.ListApplications()),
		Companies:    len(s.store.ListCompanies()),
	}
	for _, e := range s.store.LoadErrors() {
		d.LoadErrors = append(d.LoadErrors, views.LoadError{Path: e.Path, Err: e.Err})
	}
	s.render(w, r, http.StatusOK, views.SettingsPage(s.page("Settings", "settings"), d))
}

func (s *Server) reload(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if err := s.store.Reload(); err != nil {
		s.serverError(w, r, err)
		return
	}
	redirect(w, r, returnTo(r, "/settings"))
}
