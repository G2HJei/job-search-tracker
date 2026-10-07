package store

import (
	"path"
	"slices"
	"strings"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

func companyPath(id string) string { return path.Join(companyDir, id+".yaml") }

// ListCompanies returns copies of all companies sorted by name.
func (s *Store) ListCompanies() []model.Company {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Company, 0, len(s.companies))
	for _, c := range s.companies {
		out = append(out, *c)
	}
	slices.SortFunc(out, func(x, y model.Company) int {
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	return out
}

// GetCompany returns a copy of one company.
func (s *Store) GetCompany(id string) (model.Company, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.companies[id]
	if !ok {
		return model.Company{}, false
	}
	return *c, true
}

// CompanyByName finds a company by name, ignoring case and extra spaces.
func (s *Store) CompanyByName(name string) (model.Company, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.companyByName(name)
}

func (s *Store) companyByName(name string) (model.Company, bool) {
	key := strings.ToLower(model.NormalizeLine(name))
	for _, c := range s.companies {
		if strings.ToLower(model.NormalizeLine(c.Name)) == key {
			return *c, true
		}
	}
	return model.Company{}, false
}

// CreateCompany saves a new company. Its ID is the slugified name and never
// changes, even if the company is renamed.
func (s *Store) CreateCompany(c model.Company) (model.Company, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createCompany(c)
}

func (s *Store) createCompany(c model.Company) (model.Company, error) {
	c.SchemaVersion = model.SchemaVersion
	base := Slugify(c.Name, 40)
	if base == "" {
		base = "company"
	}
	c.ID = uniqueID(base, func(id string) bool {
		if _, ok := s.companies[id]; ok {
			return true
		}
		_, err := s.root.Stat(companyPath(id)) // includes files that failed to load
		return err == nil
	})
	if err := s.writeYAML(companyPath(c.ID), c); err != nil {
		return model.Company{}, err
	}
	s.companies[c.ID] = &c
	return c, nil
}

// EnsureCompany returns the company with this name, creating it if needed.
func (s *Store) EnsureCompany(name string) (c model.Company, created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.companyByName(name); ok {
		return c, false, nil
	}
	c, err = s.createCompany(model.Company{Name: model.NormalizeLine(name)})
	return c, err == nil, err
}

// UpdateCompany applies fn to a copy of the company and saves it.
func (s *Store) UpdateCompany(id string, fn func(c *model.Company) error) (model.Company, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.companies[id]
	if !ok {
		return model.Company{}, ErrNotFound
	}
	if err := s.checkUnchanged(companyPath(id)); err != nil {
		return model.Company{}, err
	}
	c := *cur
	if err := fn(&c); err != nil {
		return model.Company{}, err
	}
	c.ID, c.SchemaVersion = cur.ID, model.SchemaVersion
	if err := s.writeYAML(companyPath(id), c); err != nil {
		return model.Company{}, err
	}
	s.companies[id] = &c
	return c, nil
}
