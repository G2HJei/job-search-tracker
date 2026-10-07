package web

import (
	"cmp"
	"errors"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/store"
	"github.com/G2HJei/job-search-tracker/internal/views"
	"github.com/G2HJei/job-search-tracker/internal/views/sections"
)

// companyNames maps company IDs to names.
func (s *Server) companyNames() map[string]string {
	names := map[string]string{}
	for _, c := range s.store.ListCompanies() {
		names[c.ID] = c.Name
	}
	return names
}

// rows pairs applications with their company names.
func (s *Server) rows(apps []model.Application) []views.Row {
	names := s.companyNames()
	out := make([]views.Row, len(apps))
	for i, a := range apps {
		name, ok := names[a.CompanyID]
		if !ok {
			name = a.CompanyID
		}
		out[i] = views.Row{App: a, Company: name, CompanyKnown: ok}
	}
	return out
}

// titles maps application IDs to "Company · Job title".
func (s *Server) titles(apps []model.Application) map[string]string {
	out := map[string]string{}
	for _, r := range s.rows(apps) {
		out[r.App.ID] = r.Company + " · " + r.App.Job.Title
	}
	return out
}

func (s *Server) listApplications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cfg := s.store.Config()
	l := views.ListData{
		Config:   cfg,
		Today:    s.today(),
		Q:        strings.TrimSpace(q.Get("q")),
		Status:   q.Get("status"),
		Priority: q.Get("priority"),
		Closed:   q.Get("closed") == "1",
		Sort:     q.Get("sort"),
		Dir:      q.Get("dir"),
	}
	apps := s.store.ListApplications()
	l.Total = len(apps)
	terms := strings.Fields(strings.ToLower(l.Q))
	for _, row := range s.rows(apps) {
		a := &row.App
		switch {
		case l.Status != "" && a.Status != l.Status:
			continue
		case l.Status == "" && !l.Closed && !cfg.IsActive(a.Status):
			continue // a status filter shows closed applications too
		case l.Priority != "" && a.Priority != l.Priority:
			continue
		}
		if len(terms) > 0 {
			text := a.SearchText(row.Company)
			if !allIn(terms, text) {
				continue
			}
		}
		l.Rows = append(l.Rows, row)
	}
	sortRows(l.Rows, l.Sort, l.Dir, &cfg)
	if isHX(r) && r.Header.Get("HX-Target") == "results" {
		s.render(w, r, http.StatusOK, views.ApplicationsResults(l))
		return
	}
	p := s.page("Applications", "applications")
	p.Query = l.Q
	p.Wide = true
	s.render(w, r, http.StatusOK, views.ApplicationsPage(p, l))
}

func allIn(terms []string, text string) bool {
	for _, t := range terms {
		if !strings.Contains(text, t) {
			return false
		}
	}
	return true
}

// sortRows sorts by a column. Empty values always go last; ties are broken
// by company and ID. The default is next step date.
func sortRows(rows []views.Row, key, dir string, cfg *model.Config) {
	desc := dir == "desc"
	fold := func(a, b string) (int, bool, bool) {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b)), a == "", b == ""
	}
	dates := func(a, b model.Date) (int, bool, bool) { return a.Compare(b), a.IsZero(), b.IsZero() }
	ints := func(a, b int) (int, bool, bool) { return cmp.Compare(a, b), a == 0, b == 0 }
	slices.SortStableFunc(rows, func(x, y views.Row) int {
		a, b := &x.App, &y.App
		var c int
		var ea, eb bool
		switch key {
		case "priority":
			c, ea, eb = cmp.Compare(cfg.PriorityIndex(a.Priority), cfg.PriorityIndex(b.Priority)), a.Priority == "", b.Priority == ""
		case "status":
			c = cmp.Compare(cfg.StatusIndex(a.Status), cfg.StatusIndex(b.Status))
		case "company":
			c, ea, eb = fold(x.Company, y.Company)
		case "title":
			c, ea, eb = fold(a.Job.Title, b.Job.Title)
		case "location":
			c, ea, eb = fold(a.Job.Location, b.Job.Location)
		case "fit":
			c, ea, eb = ints(a.Job.Fit, b.Job.Fit)
		case "interest":
			c, ea, eb = ints(a.Job.Interest, b.Job.Interest)
		case "nextStep":
			c, ea, eb = fold(a.Process.NextStep, b.Process.NextStep)
		case "lastContact":
			c, ea, eb = dates(a.Process.LastContactDate, b.Process.LastContactDate)
		case "applied":
			c, ea, eb = dates(a.Process.DateApplied, b.Process.DateApplied)
		default: // nextStepDate
			c, ea, eb = dates(a.Process.NextStepDate, b.Process.NextStepDate)
		}
		if ea != eb {
			if ea {
				return 1
			}
			return -1
		}
		if desc {
			c = -c
		}
		return cmp.Or(c, strings.Compare(strings.ToLower(x.Company), strings.ToLower(y.Company)), strings.Compare(a.ID, b.ID))
	})
}

func (s *Server) newApplication(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	d := views.NewAppData{
		Config:    cfg,
		Companies: s.store.ListCompanies(),
		Company:   r.URL.Query().Get("company"),
		Priority:  cfg.DefaultPriority(),
		Status:    cfg.DefaultStatus(),
	}
	s.render(w, r, http.StatusOK, views.NewApplicationPage(s.page("New application", ""), d))
}

func (s *Server) createApplication(w http.ResponseWriter, r *http.Request) {
	f, err := newForm(r)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	cfg := s.store.Config()
	d := views.NewAppData{
		Config:     cfg,
		Companies:  s.store.ListCompanies(),
		Company:    f.line("company"),
		Title:      f.line("title"),
		PostingURL: f.line("postingUrl"),
		Priority:   f.line("priority"),
		Status:     f.line("status"),
		Errors:     model.FieldErrors{},
	}
	if d.Company == "" {
		d.Errors.Add("company", "Company is required")
	}
	if d.Title == "" {
		d.Errors.Add("title", "Job title is required")
	}
	if !model.ValidURL(d.PostingURL) {
		d.Errors.Add("postingUrl", "Must be an http:// or https:// link")
	}
	if _, ok := cfg.Priority(d.Priority); !ok {
		d.Errors.Add("priority", "Pick a priority")
	}
	if _, ok := cfg.Status(d.Status); !ok {
		d.Errors.Add("status", "Pick a status")
	}
	if len(d.Errors) > 0 {
		s.render(w, r, http.StatusUnprocessableEntity, views.NewApplicationPage(s.page("New application", ""), d))
		return
	}
	c, _, err := s.store.EnsureCompany(d.Company)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	a := model.Application{
		Priority:  d.Priority,
		CompanyID: c.ID,
		Job:       model.Job{Title: d.Title, PostingURL: d.PostingURL},
	}
	a.ApplyStatus(d.Status, s.today())
	a, err = s.store.CreateApplication(a)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	to := "/applications/" + a.ID
	if isHX(r) {
		w.Header().Set("HX-Location", to)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	redirect(w, r, to)
}

// sectionData gathers what the section cards need.
func (s *Server) sectionData(a model.Application) sections.Data {
	cfg := s.store.Config()
	d := sections.Data{App: a, Config: cfg, Today: s.today(), Errors: model.FieldErrors{}}
	d.Company, d.CompanyFound = s.store.GetCompany(a.CompanyID)
	if !d.CompanyFound {
		d.Company = model.Company{Name: a.CompanyID}
	}
	for _, other := range s.store.ListApplications() {
		if other.CompanyID == a.CompanyID {
			d.CompanyApps++
		}
	}
	if files, err := s.store.ListFiles(a.ID); err == nil {
		for _, f := range files {
			d.Files = append(d.Files, sections.FileItem{Name: f.Name, Size: f.Size})
		}
	}
	if cvs, err := s.store.ListCVs(); err == nil {
		for _, f := range cvs {
			d.CVs = append(d.CVs, f.Name)
		}
	}
	d.Companies = s.store.ListCompanies()
	d.Industries = industries(d.Companies)
	return d
}

func industries(companies []model.Company) []string {
	var out []string
	for _, c := range companies {
		if c.Industry != "" && !slices.Contains(out, c.Industry) {
			out = append(out, c.Industry)
		}
	}
	slices.Sort(out)
	return out
}

// detailData gathers everything for the application page.
func (s *Server) detailData(a model.Application) views.DetailData {
	d := views.DetailData{Base: s.sectionData(a)}
	_, d.Warnings = model.ValidateApplication(&a, &d.Base.Config)
	if ext := strings.ToLower(path.Ext(a.Job.PostingCopy)); ext == ".md" || ext == ".txt" {
		if f, err := s.store.OpenFile(a.ID, a.Job.PostingCopy); err == nil {
			b, _ := io.ReadAll(io.LimitReader(f, 512<<10))
			f.Close()
			d.PostingText = string(b)
		}
	}
	return d
}

func (s *Server) detailPage(a model.Application, d views.DetailData) templ.Component {
	title := a.Job.Title
	if d.Base.CompanyFound {
		title = d.Base.Company.Name + " · " + title
	}
	return views.DetailPage(s.page(title, "applications"), d)
}

func (s *Server) showApplication(w http.ResponseWriter, r *http.Request) {
	a, ok := s.store.GetApplication(r.PathValue("id"))
	if !ok {
		s.notFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, s.detailPage(a, s.detailData(a)))
}

// lookupSection validates the path and loads the application.
func (s *Server) lookupSection(w http.ResponseWriter, r *http.Request) (model.Application, string, bool) {
	name := r.PathValue("section")
	a, ok := s.store.GetApplication(r.PathValue("id"))
	if !ok || !sections.Valid(name) {
		s.notFound(w, r)
		return a, name, false
	}
	return a, name, true
}

func (s *Server) sectionView(w http.ResponseWriter, r *http.Request) {
	a, name, ok := s.lookupSection(w, r)
	if !ok {
		return
	}
	if !isHX(r) {
		redirect(w, r, "/applications/"+a.ID+"#section-"+name)
		return
	}
	s.render(w, r, http.StatusOK, sections.View(name, s.sectionData(a)))
}

func (s *Server) sectionEdit(w http.ResponseWriter, r *http.Request) {
	a, name, ok := s.lookupSection(w, r)
	if !ok {
		return
	}
	d := s.sectionData(a)
	s.renderEdit(w, r, http.StatusOK, name, d, a)
}

// renderEdit shows a section's edit form: the partial for htmx, or the
// whole page with that section in edit mode.
func (s *Server) renderEdit(w http.ResponseWriter, r *http.Request, status int, name string, d sections.Data, stored model.Application) {
	if isHX(r) {
		s.render(w, r, status, sections.Edit(name, d))
		return
	}
	dd := s.detailData(stored)
	dd.EditName, dd.EditData = name, d
	s.render(w, r, status, s.detailPage(stored, dd))
}

// copySection copies one section from src to dst.
func copySection(name string, dst, src *model.Application) {
	switch name {
	case "job":
		dst.Job = src.Job
	case "contacts":
		dst.Contacts = src.Contacts
	case "process":
		dst.Process = src.Process
	case "compensation":
		dst.Compensation = src.Compensation
	case "other":
		dst.Other = src.Other
	}
}

const conflictMsg = "This section was changed elsewhere since you opened it. Your version is shown below: " +
	"Save again to overwrite the other change, or Cancel to see it."

func (s *Server) sectionSave(w http.ResponseWriter, r *http.Request) {
	a, name, ok := s.lookupSection(w, r)
	if !ok {
		return
	}
	f, err := newForm(r)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	rev := r.PostFormValue("rev")
	if name == "company" {
		s.saveCompanySection(w, r, a, f, rev)
		return
	}

	edited := a.Clone()
	var errs model.FieldErrors
	switch name {
	case "job":
		edited.Job = bindJob(f)
		errs = model.ValidateJob(&edited.Job)
	case "contacts":
		edited.Contacts = bindContacts(f)
		errs = model.ValidateContacts(&edited.Contacts)
	case "process":
		edited.Process = bindProcess(f)
		errs = model.ValidateProcess(&edited.Process)
	case "compensation":
		edited.Compensation = bindCompensation(f)
		errs = model.ValidateCompensation(&edited.Compensation)
	case "other":
		edited.Other = bindOther(f)
	}
	f.errs.Merge(errs) // parse errors take precedence over validation messages
	if len(f.errs) > 0 {
		d := s.sectionData(edited)
		d.Errors, d.Rev = f.errs, rev
		s.renderEdit(w, r, http.StatusUnprocessableEntity, name, d, a)
		return
	}

	updated, err := s.store.UpdateApplication(a.ID, func(cur *model.Application) error {
		if rev != "" && sections.Rev(name, *cur, model.Company{}) != rev {
			return store.ErrConflict
		}
		copySection(name, cur, &edited)
		return nil
	})
	if err != nil {
		s.sectionConflict(w, r, err, name, edited)
		return
	}
	if !isHX(r) {
		redirect(w, r, "/applications/"+a.ID+"#section-"+name)
		return
	}
	toast(w, "Saved")
	s.render(w, r, http.StatusOK, sections.View(name, s.sectionData(updated)))
}

// sectionConflict re-shows the user's edits with an explanation when the
// data changed underneath them.
func (s *Server) sectionConflict(w http.ResponseWriter, r *http.Request, err error, name string, edited model.Application) {
	cur, ok := s.store.GetApplication(edited.ID)
	if !ok || !(errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrExternalChange)) {
		s.storeError(w, r, err)
		return
	}
	d := s.sectionData(edited)
	c, _ := s.store.GetCompany(cur.CompanyID)
	if errors.Is(err, store.ErrConflict) {
		d.Error = conflictMsg
		d.Rev = sections.Rev(name, cur, c) // saving again overwrites
	} else {
		d.Error = "The file was changed on disk since it was loaded. Copy anything you need from below, " +
			"then use Reload from disk (Settings) and edit again."
		d.Rev = sections.Rev(name, edited, d.Company)
	}
	s.renderEdit(w, r, http.StatusConflict, name, d, cur)
}

func (s *Server) saveCompanySection(w http.ResponseWriter, r *http.Request, a model.Application, f *form, rev string) {
	c := bindCompany(f)
	f.errs.Merge(model.ValidateCompany(&c))
	if other, ok := s.store.CompanyByName(c.Name); ok && other.ID != a.CompanyID {
		f.errs.Add("company.name", `Another company already has this name. To switch, use "Link this application to a different company".`)
	}
	if len(f.errs) > 0 {
		d := s.sectionData(a)
		c.ID = d.Company.ID
		d.Company, d.Errors, d.Rev = c, f.errs, rev
		s.renderEdit(w, r, http.StatusUnprocessableEntity, "company", d, a)
		return
	}
	var err error
	if _, found := s.store.GetCompany(a.CompanyID); found {
		_, err = s.store.UpdateCompany(a.CompanyID, func(cur *model.Company) error {
			if rev != "" && model.Rev(*cur) != rev {
				return store.ErrConflict
			}
			*cur = c
			return nil
		})
	} else {
		var created model.Company
		if created, err = s.store.CreateCompany(c); err == nil {
			a, err = s.store.UpdateApplication(a.ID, func(x *model.Application) error {
				x.CompanyID = created.ID
				return nil
			})
		}
	}
	if err != nil {
		if errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrExternalChange) {
			d := s.sectionData(a)
			cur, _ := s.store.GetCompany(a.CompanyID)
			c.ID = cur.ID
			d.Company, d.Rev = c, model.Rev(cur)
			d.Error = conflictMsg
			if errors.Is(err, store.ErrExternalChange) {
				d.Error = "The company file was changed on disk. Reload from disk (Settings) and edit again."
			}
			s.renderEdit(w, r, http.StatusConflict, "company", d, a)
			return
		}
		s.storeError(w, r, err)
		return
	}
	if !isHX(r) {
		redirect(w, r, "/applications/"+a.ID+"#section-company")
		return
	}
	toast(w, "Saved")
	s.render(w, r, http.StatusOK, sections.View("company", s.sectionData(a)))
}

func (s *Server) relinkCompany(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := newForm(r)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	name := f.line("company")
	if name == "" {
		redirect(w, r, "/applications/"+id+"/sections/company/edit")
		return
	}
	if _, ok := s.store.GetApplication(id); !ok {
		s.notFound(w, r)
		return
	}
	c, _, err := s.store.EnsureCompany(name)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if _, err := s.store.UpdateApplication(id, func(a *model.Application) error {
		a.CompanyID = c.ID
		return nil
	}); err != nil {
		s.storeError(w, r, err)
		return
	}
	redirect(w, r, "/applications/"+id+"#section-company")
}

func (s *Server) setStatus(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	cfg := s.store.Config()
	st, ok := cfg.Status(r.PostFormValue("status"))
	if !ok {
		s.fail(w, r, http.StatusBadRequest, "Unknown status")
		return
	}
	today := s.today()
	s.quickUpdate(w, r, "Status: "+st.Label, func(a *model.Application) error {
		a.ApplyStatus(st.ID, today)
		return nil
	})
}

func (s *Server) setPriority(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	cfg := s.store.Config()
	p, ok := cfg.Priority(r.PostFormValue("priority"))
	if !ok {
		s.fail(w, r, http.StatusBadRequest, "Unknown priority")
		return
	}
	s.quickUpdate(w, r, "Priority: "+p.Label, func(a *model.Application) error {
		a.Priority = p.ID
		return nil
	})
}

func (s *Server) touch(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	today := s.today()
	s.quickUpdate(w, r, "Last contact set to today", func(a *model.Application) error {
		a.Process.LastContactDate = today
		return nil
	})
}

// quickUpdate applies a one-click change and answers according to where it
// came from (the "ctx" field): the detail header, the decision suggestion,
// the dashboard or the board.
func (s *Server) quickUpdate(w http.ResponseWriter, r *http.Request, msg string, fn func(a *model.Application) error) {
	id := r.PathValue("id")
	before, ok := s.store.GetApplication(id)
	if !ok {
		s.notFound(w, r)
		return
	}
	after, err := s.store.UpdateApplication(id, fn)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !isHX(r) {
		redirect(w, r, returnTo(r, "/applications/"+id))
		return
	}
	toast(w, msg)
	switch r.PostFormValue("ctx") {
	case "detail":
		d := s.detailData(after)
		out := []templ.Component{views.DetailHeader(d, false)}
		if model.Rev(before.Process) != model.Rev(after.Process) {
			out = append(out, sections.OOB("process", d.Base))
		}
		s.render(w, r, http.StatusOK, out...)
	case "suggest":
		d := s.detailData(after)
		s.render(w, r, http.StatusOK, sections.View("process", d.Base), views.DetailHeader(d, true))
	case "dashboard":
		w.WriteHeader(http.StatusOK) // empty body: the item is removed
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) deleteApplication(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteApplication(r.PathValue("id")); err != nil {
		s.storeError(w, r, err)
		return
	}
	redirect(w, r, "/applications")
}

// blankRow returns an empty repeatable row for the "Add" buttons.
func (s *Server) blankRow(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	var c templ.Component
	switch r.PathValue("kind") {
	case "contact":
		prefix := r.URL.Query().Get("prefix")
		if prefix != "recruiter" && prefix != "interviewer" {
			s.fail(w, r, http.StatusBadRequest, "Unknown contact list")
			return
		}
		c = sections.ContactRow(prefix, model.Contact{}, nil, "")
	case "interview":
		c = sections.InterviewRow(model.Interview{}, cfg.InterviewTypes, "")
	case "qa":
		n, _ := strconv.Atoi(r.URL.Query().Get("rounds"))
		c = sections.QARow(model.QA{}, sections.Rounds(min(max(n, 3), 20)), "")
	case "material":
		c = sections.MaterialRow("", "")
	default:
		s.notFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, c)
}
