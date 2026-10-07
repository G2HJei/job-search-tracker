package model

import (
	"cmp"
	"slices"
)

// StaleApp is an active application that needs a follow-up.
type StaleApp struct {
	App   *Application
	Since Date // latest of last contact and date applied
	Days  int  // days since then
}

// StatusCount is the number of applications with a status.
type StatusCount struct {
	Status Status
	Known  bool // false if the status is not in config.yaml
	Count  int
}

// Funnel counts applications that reached each stage.
type Funnel struct {
	Applied, Screening, Interview, Offer int
}

// Dashboard is everything shown on the home page.
type Dashboard struct {
	Today    Date
	Overdue  []AppEvent
	Upcoming []DayEvents
	Stale    []StaleApp
	Pipeline []StatusCount
	MaxCount int
	Active   int
	Total    int
	Funnel   Funnel
}

// BuildDashboard derives the dashboard from all applications.
func BuildDashboard(apps []Application, cfg *Config, today Date) Dashboard {
	d := Dashboard{
		Today:    today,
		Overdue:  Overdue(apps, cfg, today),
		Upcoming: Upcoming(apps, cfg, today, cfg.UpcomingDays),
		Stale:    Stale(apps, cfg, today),
		Total:    len(apps),
	}
	d.Pipeline = Pipeline(apps, cfg)
	for _, sc := range d.Pipeline {
		d.MaxCount = max(d.MaxCount, sc.Count)
	}
	for i := range apps {
		if cfg.IsActive(apps[i].Status) {
			d.Active++
		}
	}
	d.Funnel = BuildFunnel(apps)
	return d
}

// Stale returns active applications whose latest contact (or application
// date) is at least staleAfterDays old and that have nothing scheduled.
func Stale(apps []Application, cfg *Config, today Date) []StaleApp {
	var out []StaleApp
	for i := range apps {
		a := &apps[i]
		since := a.LatestContact()
		if !cfg.IsActive(a.Status) || since.IsZero() {
			continue
		}
		days := since.DaysUntil(today)
		if days < cfg.StaleAfterDays {
			continue
		}
		scheduled := slices.ContainsFunc(Events(*a), func(e Event) bool { return !e.Date().Before(today) })
		if !scheduled {
			out = append(out, StaleApp{App: a, Since: since, Days: days})
		}
	}
	slices.SortStableFunc(out, func(x, y StaleApp) int {
		return cmp.Or(cmp.Compare(y.Days, x.Days), cmp.Compare(x.App.ID, y.App.ID))
	})
	return out
}

// Pipeline counts applications per status in config order. Statuses that
// are not in config.yaml are appended at the end.
func Pipeline(apps []Application, cfg *Config) []StatusCount {
	counts := map[string]int{}
	var unknown []string
	for _, a := range apps {
		if _, ok := cfg.Status(a.Status); !ok && counts[a.Status] == 0 {
			unknown = append(unknown, a.Status)
		}
		counts[a.Status]++
	}
	var out []StatusCount
	for _, s := range cfg.Statuses {
		out = append(out, StatusCount{Status: s, Known: true, Count: counts[s.ID]})
	}
	slices.Sort(unknown)
	for _, id := range unknown {
		out = append(out, StatusCount{Status: Status{ID: id, Label: id}, Count: counts[id]})
	}
	return out
}

// stageRank is how far an application got: 0 not applied, 1 applied,
// 2 screening, 3 interviewing, 4 offer. It uses the status (for the default
// status ids) and the recorded dates, so closed applications still count
// for the stages they reached.
func stageRank(a *Application) int {
	rank := 0
	switch a.Status {
	case "applied", "rejected", "ghosted":
		rank = 1
	case "screening":
		rank = 2
	case "interviewing":
		rank = 3
	case "offer", "accepted":
		rank = 4
	}
	p := &a.Process
	if !p.DateApplied.IsZero() {
		rank = max(rank, 1)
	}
	if !p.ScreeningDate.IsZero() {
		rank = max(rank, 2)
	}
	if len(p.Interviews) > 0 {
		rank = max(rank, 3)
	}
	if !p.OfferDate.IsZero() || !a.Compensation.Offered.IsZero() {
		rank = max(rank, 4)
	}
	return rank
}

// BuildFunnel counts how many applications reached each stage.
func BuildFunnel(apps []Application) Funnel {
	var f Funnel
	for i := range apps {
		r := stageRank(&apps[i])
		if r >= 1 {
			f.Applied++
		}
		if r >= 2 {
			f.Screening++
		}
		if r >= 3 {
			f.Interview++
		}
		if r >= 4 {
			f.Offer++
		}
	}
	return f
}

// Percent returns n as a whole-number percentage of total.
func Percent(n, total int) int {
	if total == 0 {
		return 0
	}
	return (n*100 + total/2) / total
}
