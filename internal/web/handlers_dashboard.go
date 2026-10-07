package web

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/views"
)

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	apps := s.store.ListApplications()
	d := views.DashboardData{
		Dashboard: model.BuildDashboard(apps, &cfg, s.today()),
		Config:    cfg,
		Titles:    s.titles(apps),
	}
	s.render(w, r, http.StatusOK, views.DashboardPage(s.page("Dashboard", "dashboard"), d))
}

func (s *Server) board(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	b := views.BoardData{Config: cfg, Today: s.today()}
	byStatus := map[string][]views.Row{}
	for _, row := range s.rows(s.store.ListApplications()) {
		byStatus[row.App.Status] = append(byStatus[row.App.Status], row)
	}
	for _, st := range cfg.Statuses {
		b.Columns = append(b.Columns, views.BoardColumn{Status: st, Known: true, Rows: byStatus[st.ID]})
		delete(byStatus, st.ID)
	}
	var unknown []string
	for id := range byStatus {
		unknown = append(unknown, id)
	}
	slices.Sort(unknown)
	for _, id := range unknown {
		b.Columns = append(b.Columns, views.BoardColumn{Status: model.Status{ID: id, Label: id}, Rows: byStatus[id]})
	}
	for _, col := range b.Columns {
		slices.SortStableFunc(col.Rows, func(x, y views.Row) int {
			a, b := &x.App, &y.App
			return cmp.Or(
				cmp.Compare(cfg.PriorityIndex(a.Priority), cfg.PriorityIndex(b.Priority)),
				compareDatesEmptyLast(a.Process.NextStepDate, b.Process.NextStepDate),
				cmp.Compare(x.Company, y.Company),
			)
		})
	}
	p := s.page("Board", "board")
	p.Wide, p.Board = true, true
	s.render(w, r, http.StatusOK, views.BoardPage(p, b))
}

func compareDatesEmptyLast(a, b model.Date) int {
	switch {
	case a.IsZero() && b.IsZero():
		return 0
	case a.IsZero():
		return 1
	case b.IsZero():
		return -1
	}
	return a.Compare(b)
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Config()
	apps := s.store.ListApplications()
	today := s.today()
	c := views.CalendarData{
		Config:  cfg,
		Today:   today,
		Overdue: model.Overdue(apps, &cfg, today),
		Days:    model.Upcoming(apps, &cfg, today, -1),
		Titles:  s.titles(apps),
		ICSURL:  "http://" + r.Host + "/calendar.ics",
	}
	s.render(w, r, http.StatusOK, views.CalendarPage(s.page("Calendar", "calendar"), c))
}
