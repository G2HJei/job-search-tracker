package demo

import (
	"log/slog"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/store"
)

type summary struct {
	overdue, upcoming, stale []string
}

func dashboardFor(t *testing.T, today model.Date) summary {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	if err := CopyTo(dir, today); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, store.Options{Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return today.Time(time.Local) }})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if errs := st.LoadErrors(); len(errs) != 0 {
		t.Fatalf("sample data has load errors: %+v", errs)
	}
	cfg := st.Config()
	d := model.BuildDashboard(st.ListApplications(), &cfg, today)
	var s summary
	for _, e := range d.Overdue {
		s.overdue = append(s.overdue, e.App.ID+" "+string(e.Kind))
	}
	for _, day := range d.Upcoming {
		for _, e := range day.Events {
			s.upcoming = append(s.upcoming, string(e.Kind))
		}
	}
	for _, a := range d.Stale {
		s.stale = append(s.stale, a.App.ID)
	}
	return s
}

// TestDashboardForDemoData pins the scenario the sample data was written for
// (see the reference date Today).
func TestDashboardForDemoData(t *testing.T) {
	s := dashboardFor(t, Today)
	wantOverdue := []string{
		"2026-09-08-harbourline-logistics-senior-software-engineer-routing follow-up",
		"2026-09-11-orbweaver-security-backend-engineer-go next-step",
		"2026-09-29-quillfeather-labs-platform-engineer follow-up",
	}
	if !slices.Equal(s.overdue, wantOverdue) {
		t.Errorf("overdue = %v\nwant %v", s.overdue, wantOverdue)
	}
	wantUpcoming := []string{"offer", "next-step", "screening", "next-step", "next-step", "take-home",
		"interview", "offer-response", "interview", "follow-up"}
	if !slices.Equal(s.upcoming, wantUpcoming) {
		t.Errorf("upcoming = %v\nwant %v", s.upcoming, wantUpcoming)
	}
	wantStale := []string{
		"2026-09-08-harbourline-logistics-senior-software-engineer-routing",
		"2026-09-11-orbweaver-security-backend-engineer-go",
	}
	if !slices.Equal(s.stale, wantStale) {
		t.Errorf("stale = %v\nwant %v", s.stale, wantStale)
	}
}

// TestCopyToShiftsDates checks that --demo looks the same on any day.
func TestCopyToShiftsDates(t *testing.T) {
	base := dashboardFor(t, Today)
	later := dashboardFor(t, Today.AddDays(45))
	if len(later.overdue) != len(base.overdue) || !slices.Equal(later.upcoming, base.upcoming) || len(later.stale) != len(base.stale) {
		t.Fatalf("shifted dashboard differs:\n%+v\n%+v", later, base)
	}
}
