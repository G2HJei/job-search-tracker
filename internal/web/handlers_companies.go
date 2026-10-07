package web

import (
	"errors"
	"net/http"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/store"
	"github.com/G2HJei/job-search-tracker/internal/views"
)

func (s *Server) listCompanies(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	apps := s.store.ListApplications()
	var rows []views.CompanyRow
	for _, c := range s.store.ListCompanies() {
		row := views.CompanyRow{Company: c}
		for _, a := range apps {
			if a.CompanyID == c.ID {
				row.Total++
				if cfg.IsActive(a.Status) {
					row.Active++
				}
			}
		}
		rows = append(rows, row)
	}
	s.render(w, r, http.StatusOK, views.CompaniesPage(s.page("Companies", "companies"), cfg, rows))
}

func (s *Server) showCompany(w http.ResponseWriter, r *http.Request) {
	c, ok := s.store.GetCompany(r.PathValue("id"))
	if !ok {
		s.notFound(w, r)
		return
	}
	var apps []model.Application
	for _, a := range s.store.ListApplications() {
		if a.CompanyID == c.ID {
			apps = append(apps, a)
		}
	}
	d := views.CompanyData{Company: c, Config: s.store.Config(), Today: s.today(), Rows: s.rows(apps)}
	s.render(w, r, http.StatusOK, views.CompanyPage(s.page(c.Name, "companies"), d))
}

func (s *Server) companyForm(c model.Company, isNew bool) views.CompanyFormData {
	return views.CompanyFormData{
		Company:    c,
		Config:     s.store.Config(),
		Industries: industries(s.store.ListCompanies()),
		Errors:     model.FieldErrors{},
		New:        isNew,
	}
}

func (s *Server) newCompany(w http.ResponseWriter, r *http.Request) {
	d := s.companyForm(model.Company{}, true)
	s.render(w, r, http.StatusOK, views.CompanyFormPage(s.page("New company", "companies"), d))
}

func (s *Server) editCompany(w http.ResponseWriter, r *http.Request) {
	c, ok := s.store.GetCompany(r.PathValue("id"))
	if !ok {
		s.notFound(w, r)
		return
	}
	d := s.companyForm(c, false)
	d.Rev = model.Rev(c)
	s.render(w, r, http.StatusOK, views.CompanyFormPage(s.page("Edit "+c.Name, "companies"), d))
}

// bindCompanyForm reads and validates a company form. id is the company
// being edited ("" for a new one), used to allow keeping its own name.
func (s *Server) bindCompanyForm(r *http.Request, id string) (model.Company, model.FieldErrors, error) {
	f, err := newForm(r)
	if err != nil {
		return model.Company{}, nil, err
	}
	c := bindCompany(f)
	f.errs.Merge(model.ValidateCompany(&c))
	if other, ok := s.store.CompanyByName(c.Name); ok && other.ID != id {
		f.errs.Add("company.name", "A company with this name already exists")
	}
	return c, f.errs, nil
}

func (s *Server) createCompany(w http.ResponseWriter, r *http.Request) {
	c, errs, err := s.bindCompanyForm(r, "")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if len(errs) > 0 {
		d := s.companyForm(c, true)
		d.Errors = errs
		s.render(w, r, http.StatusUnprocessableEntity, views.CompanyFormPage(s.page("New company", "companies"), d))
		return
	}
	c, err = s.store.CreateCompany(c)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	redirect(w, r, "/companies/"+c.ID)
}

func (s *Server) updateCompany(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur, ok := s.store.GetCompany(id)
	if !ok {
		s.notFound(w, r)
		return
	}
	c, errs, err := s.bindCompanyForm(r, id)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	rev := r.PostFormValue("rev")
	c.ID = id
	d := s.companyForm(c, false)
	d.Rev = rev
	if len(errs) > 0 {
		d.Errors = errs
		s.render(w, r, http.StatusUnprocessableEntity, views.CompanyFormPage(s.page("Edit "+cur.Name, "companies"), d))
		return
	}
	_, err = s.store.UpdateCompany(id, func(x *model.Company) error {
		if rev != "" && model.Rev(*x) != rev {
			return store.ErrConflict
		}
		*x = c
		return nil
	})
	switch {
	case errors.Is(err, store.ErrConflict):
		latest, _ := s.store.GetCompany(id)
		d.Rev = model.Rev(latest)
		d.Error = "This company was changed elsewhere since you opened it. Save again to overwrite, or Cancel to see the other change."
		s.render(w, r, http.StatusConflict, views.CompanyFormPage(s.page("Edit "+cur.Name, "companies"), d))
		return
	case err != nil:
		s.storeError(w, r, err)
		return
	}
	redirect(w, r, "/companies/"+id)
}
