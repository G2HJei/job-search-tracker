package store

import (
	"cmp"
	"errors"
	"io/fs"
	"path"
	"slices"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

func appPath(id string) string { return path.Join(appDir, id, appFile) }

// ListApplications returns copies of all applications, newest first.
func (s *Store) ListApplications() []model.Application {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Application, 0, len(s.apps))
	for _, a := range s.apps {
		out = append(out, a.Clone())
	}
	slices.SortFunc(out, func(x, y model.Application) int {
		return cmp.Or(y.CreatedAt.Compare(x.CreatedAt), cmp.Compare(y.ID, x.ID))
	})
	return out
}

// GetApplication returns a copy of one application.
func (s *Store) GetApplication(id string) (model.Application, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.apps[id]
	if !ok {
		return model.Application{}, false
	}
	return a.Clone(), true
}

// CreateApplication assigns an ID and timestamps, then saves a new
// application. The ID is YYYY-MM-DD-<company>-<title> and never changes.
func (s *Store) CreateApplication(a model.Application) (model.Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.timestamp(time.Time{})
	a = a.Clone()
	a.SchemaVersion = model.SchemaVersion
	a.CreatedAt, a.UpdatedAt = now, now
	title := Slugify(a.Job.Title, 40)
	if title == "" {
		title = "job"
	}
	base := model.DateOf(now).String() + "-" + Slugify(a.CompanyID, 40) + "-" + title
	a.ID = uniqueID(base, func(id string) bool {
		if _, ok := s.apps[id]; ok {
			return true
		}
		_, err := s.root.Stat(path.Join(appDir, id)) // includes folders that failed to load
		return err == nil
	})
	if err := s.root.Mkdir(path.Join(appDir, a.ID), 0o755); err != nil {
		return model.Application{}, err
	}
	if err := s.writeYAML(appPath(a.ID), a); err != nil {
		s.root.Remove(path.Join(appDir, a.ID))
		return model.Application{}, err
	}
	s.apps[a.ID] = &a
	return a.Clone(), nil
}

// UpdateApplication applies fn to a copy of the application and saves it.
// fn runs under the store lock, so it can compare a form's revision against
// the current data and return ErrConflict. The ID and creation time cannot
// be changed.
func (s *Store) UpdateApplication(id string, fn func(a *model.Application) error) (model.Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.apps[id]
	if !ok {
		return model.Application{}, ErrNotFound
	}
	if err := s.checkUnchanged(appPath(id)); err != nil {
		return model.Application{}, err
	}
	a := cur.Clone()
	if err := fn(&a); err != nil {
		return model.Application{}, err
	}
	a.ID, a.CreatedAt, a.SchemaVersion = cur.ID, cur.CreatedAt, model.SchemaVersion
	a.UpdatedAt = s.timestamp(cur.UpdatedAt)
	if err := s.writeYAML(appPath(id), a); err != nil {
		return model.Application{}, err
	}
	s.apps[id] = &a
	return a.Clone(), nil
}

// DeleteApplication moves the application folder to .trash.
func (s *Store) DeleteApplication(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.apps[id]; !ok {
		return ErrNotFound
	}
	if err := renameRetry(s.root, path.Join(appDir, id), s.trashName(id)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			delete(s.apps, id)
			return ErrNotFound
		}
		return err
	}
	delete(s.apps, id)
	delete(s.sigs, appPath(id))
	return nil
}
