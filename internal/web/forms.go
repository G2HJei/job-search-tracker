package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

// form reads and tidies posted values, collecting parse errors by field key.
type form struct {
	r    *http.Request
	errs model.FieldErrors
}

// newForm parses the posted form.
func newForm(r *http.Request) (*form, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return &form{r: r, errs: model.FieldErrors{}}, nil
}

// at returns the i-th value posted under name. Repeatable rows repeat the
// same input names, so row i's fields are at index i of each name.
func (f *form) at(name string, i int) string {
	if vals := f.r.PostForm[name]; i < len(vals) {
		return vals[i]
	}
	return ""
}

func (f *form) count(name string) int { return len(f.r.PostForm[name]) }

func (f *form) line(name string) string { return model.NormalizeLine(f.r.PostFormValue(name)) }

func (f *form) text(name string) string { return model.NormalizeText(f.r.PostFormValue(name)) }

func (f *form) date(name, key string) model.Date {
	return f.parseDate(f.r.PostFormValue(name), key)
}

func (f *form) parseDate(s, key string) model.Date {
	var d model.Date
	if err := d.ParseForm(s); err != nil {
		f.errs.Add(key, "Use a date like 2026-10-14")
	}
	return d
}

func (f *form) dateTime(name, key string) model.DateTime {
	return f.parseDateTime(f.r.PostFormValue(name), key)
}

func (f *form) parseDateTime(s, key string) model.DateTime {
	var dt model.DateTime
	if err := dt.ParseForm(s); err != nil {
		f.errs.Add(key, "Use a date and time like 2026-10-14 14:00")
	}
	return dt
}

// number parses a whole number, allowing "80,000" and "80 000".
func (f *form) number(name, key string) int {
	s := strings.NewReplacer(",", "", " ", "", "_", "").Replace(f.r.PostFormValue(name))
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		f.errs.Add(key, "Must be a whole number")
	}
	return n
}

// salary reads <prefix>_min/_max/_currency/_period/_note. A range with no
// amounts and no note is empty, whatever the currency and period say.
func (f *form) salary(prefix, key string) model.SalaryRange {
	s := model.SalaryRange{
		Min:      f.number(prefix+"_min", key),
		Max:      f.number(prefix+"_max", key),
		Currency: strings.ToUpper(f.line(prefix + "_currency")),
		Period:   f.line(prefix + "_period"),
		Note:     f.line(prefix + "_note"),
	}
	if s.IsZero() {
		return model.SalaryRange{}
	}
	return s
}

func (f *form) contactAt(prefix string, i int) model.Contact {
	return model.Contact{
		Name:     model.NormalizeLine(f.at(prefix+"_name", i)),
		Role:     model.NormalizeLine(f.at(prefix+"_role", i)),
		Email:    model.NormalizeLine(f.at(prefix+"_email", i)),
		Phone:    model.NormalizeLine(f.at(prefix+"_phone", i)),
		LinkedIn: model.NormalizeLine(f.at(prefix+"_linkedin", i)),
		Notes:    model.NormalizeText(f.at(prefix+"_notes", i)),
	}
}

// contact reads a single contact; a blank one is nil.
func (f *form) contact(prefix string) *model.Contact {
	c := f.contactAt(prefix, 0)
	if c.IsZero() {
		return nil
	}
	return &c
}

// contacts reads repeatable contact rows, skipping blank ones.
func (f *form) contacts(prefix string) []model.Contact {
	var out []model.Contact
	for i := range f.count(prefix + "_name") {
		if c := f.contactAt(prefix, i); !c.IsZero() {
			out = append(out, c)
		}
	}
	return out
}

func splitNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ",") {
		if n = model.NormalizeLine(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func bindCompany(f *form) model.Company {
	return model.Company{
		Name:     f.line("name"),
		Industry: f.line("industry"),
		Size:     f.line("size"),
		HQ:       f.line("hq"),
		Website:  f.line("website"),
		Notes:    f.text("notes"),
	}
}

func bindJob(f *form) model.Job {
	j := model.Job{
		Title:            f.line("title"),
		PostingURL:       f.line("postingUrl"),
		PostingCopy:      f.line("postingCopy"),
		JobID:            f.line("jobId"),
		Department:       f.line("department"),
		Location:         f.line("location"),
		KeyRequirements:  f.text("keyRequirements"),
		Fit:              f.number("fit", "job.fit"),
		Interest:         f.number("interest", "job.interest"),
		Channel:          f.line("channel"),
		CVVersion:        f.line("cvVersion"),
		AdvertisedSalary: f.salary("advertisedSalary", "job.advertisedSalary"),
	}
	for _, m := range f.r.PostForm["material"] {
		if m = model.NormalizeLine(m); m != "" {
			j.ExtraMaterials = append(j.ExtraMaterials, m)
		}
	}
	return j
}

func bindContacts(f *form) model.Contacts {
	return model.Contacts{
		Referrer:      f.contact("referrer"),
		Recruiters:    f.contacts("recruiter"),
		HiringManager: f.contact("hm"),
		Interviewers:  f.contacts("interviewer"),
	}
}

func bindProcess(f *form) model.Process {
	p := model.Process{
		NextStep:              f.line("nextStep"),
		NextStepDate:          f.date("nextStepDate", "process.nextStepDate"),
		LastContactDate:       f.date("lastContactDate", "process.lastContactDate"),
		DateApplied:           f.date("dateApplied", "process.dateApplied"),
		ScreeningDate:         f.dateTime("screeningDate", "process.screeningDate"),
		FollowUpDate:          f.date("followUpDate", "process.followUpDate"),
		OfferDate:             f.date("offerDate", "process.offerDate"),
		OfferResponseDeadline: f.date("offerResponseDeadline", "process.offerResponseDeadline"),
		TakeHome: model.TakeHome{
			Details:  f.text("takeHomeDetails"),
			Deadline: f.dateTime("takeHomeDeadline", "process.takeHome.deadline"),
		},
		Decision:          f.line("decision"),
		RejectionFeedback: f.text("rejectionFeedback"),
	}
	for i := range f.count("interview_date") {
		date, typ := f.at("interview_date", i), model.NormalizeLine(f.at("interview_type", i))
		names, notes := splitNames(f.at("interview_interviewers", i)), model.NormalizeText(f.at("interview_notes", i))
		if strings.TrimSpace(date) == "" && typ == "" && len(names) == 0 && notes == "" {
			continue
		}
		key := fmt.Sprintf("process.interviews.%d.date", len(p.Interviews))
		p.Interviews = append(p.Interviews, model.Interview{
			Date: f.parseDateTime(date, key), Type: typ, Interviewers: names, Notes: notes,
		})
	}
	for i := range f.count("qa_question") {
		q := model.QA{Question: model.NormalizeLine(f.at("qa_question", i)), Answer: model.NormalizeText(f.at("qa_answer", i))}
		if q.Question == "" && q.Answer == "" {
			continue
		}
		q.Round, _ = strconv.Atoi(f.at("qa_round", i))
		p.QA = append(p.QA, q)
	}
	return p
}

func bindCompensation(f *form) model.Compensation {
	return model.Compensation{
		Asked:        f.salary("asked", "compensation.asked"),
		Offered:      f.salary("offered", "compensation.offered"),
		ContractType: f.line("contractType"),
		Bonus:        f.line("bonus"),
		Equity:       f.line("equity"),
		Benefits:     f.text("benefits"),
		NoticePeriod: f.line("noticePeriod"),
		StartDate:    f.date("startDate", "compensation.startDate"),
	}
}

func bindOther(f *form) model.Other {
	return model.Other{
		LessonsLearned: f.text("lessonsLearned"),
		Notes:          f.text("notes"),
	}
}
