package model

import (
	"slices"
	"testing"
)

var today = NewDate(2026, 10, 7)

func TestValidateApplication(t *testing.T) {
	cfg := DefaultConfig()
	valid := Application{
		CompanyID: "acme", Status: "applied", Priority: "high",
		Job: Job{Title: "Engineer", PostingURL: "https://acme.example/jobs/1", Fit: 4,
			ExtraMaterials: []string{"cover-letter.pdf", "https://github.com/me"}},
	}
	if errs, warns := ValidateApplication(&valid, &cfg); len(errs) != 0 || len(warns) != 0 {
		t.Fatalf("valid application: errs=%v warns=%v", errs, warns)
	}

	tests := []struct {
		name  string
		edit  func(a *Application)
		field string
	}{
		{"title required", func(a *Application) { a.Job.Title = "  " }, "job.title"},
		{"company required", func(a *Application) { a.CompanyID = "" }, "companyId"},
		{"fit range", func(a *Application) { a.Job.Fit = 6 }, "job.fit"},
		{"interest range", func(a *Application) { a.Job.Interest = -1 }, "job.interest"},
		{"posting url scheme", func(a *Application) { a.Job.PostingURL = "javascript:alert(1)" }, "job.postingUrl"},
		{"posting url relative", func(a *Application) { a.Job.PostingURL = "acme.example/jobs" }, "job.postingUrl"},
		{"material scheme", func(a *Application) { a.Job.ExtraMaterials = []string{"ftp://x/y"} }, "job.extraMaterials.0"},
		{"salary min>max", func(a *Application) { a.Job.AdvertisedSalary = SalaryRange{Min: 90, Max: 80} }, "job.advertisedSalary"},
		{"offered min>max", func(a *Application) { a.Compensation.Offered = SalaryRange{Min: 90, Max: 80} }, "compensation.offered"},
		{"contact name", func(a *Application) { a.Contacts.Recruiters = []Contact{{Name: "A"}, {Email: "x@y"}} }, "contacts.recruiters.1.name"},
		{"contact linkedin", func(a *Application) { a.Contacts.Referrer = &Contact{Name: "A", LinkedIn: "linkedin"} }, "contacts.referrer.linkedin"},
		{"qa question", func(a *Application) { a.Process.QA = []QA{{Answer: "x"}} }, "process.qa.0.question"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := valid.Clone()
			tt.edit(&a)
			errs, _ := ValidateApplication(&a, &cfg)
			if errs.Get(tt.field) == "" {
				t.Fatalf("want error on %s, got %v", tt.field, errs)
			}
		})
	}

	t.Run("unknown status is a warning", func(t *testing.T) {
		a := valid.Clone()
		a.Status = "on-hold"
		errs, warns := ValidateApplication(&a, &cfg)
		if len(errs) != 0 || warns.Get("status") == "" {
			t.Fatalf("errs=%v warns=%v", errs, warns)
		}
	})
}

func TestCloneIsDeep(t *testing.T) {
	a := Application{
		Job:      Job{ExtraMaterials: []string{"a"}},
		Contacts: Contacts{Referrer: &Contact{Name: "R"}, Recruiters: []Contact{{Name: "x"}}},
		Process:  Process{Interviews: []Interview{{Interviewers: []string{"i"}}}, QA: []QA{{Question: "q"}}},
	}
	c := a.Clone()
	c.Job.ExtraMaterials[0] = "b"
	c.Contacts.Referrer.Name = "changed"
	c.Contacts.Recruiters[0].Name = "y"
	c.Process.Interviews[0].Interviewers[0] = "j"
	c.Process.QA[0].Question = "z"
	if a.Job.ExtraMaterials[0] != "a" || a.Contacts.Referrer.Name != "R" || a.Contacts.Recruiters[0].Name != "x" ||
		a.Process.Interviews[0].Interviewers[0] != "i" || a.Process.QA[0].Question != "q" {
		t.Fatalf("clone shares memory: %+v", a)
	}
}

func TestEvents(t *testing.T) {
	a := Application{ID: "x", Process: Process{
		NextStep:     "Tech interview",
		NextStepDate: NewDate(2026, 10, 14),
		FollowUpDate: NewDate(2026, 10, 16),
		Interviews: []Interview{
			{Date: NewDateTime(2026, 10, 14, 14, 0), Type: "Technical"},
			{}, // no date: skipped
			{Date: NewDateTime(2026, 10, 20, 9, 0)},
		},
		TakeHome: TakeHome{Deadline: NewDateTime(2026, 10, 12, 17, 0)},
	}, Compensation: Compensation{StartDate: NewDate(2026, 11, 2)}}
	evs := Events(a)
	var labels []string
	for _, e := range evs {
		labels = append(labels, e.Label)
	}
	want := []string{"Next step: Tech interview", "Follow up", "Interview 1 · Technical", "Interview 3", "Take-home deadline", "Start date"}
	if !slices.Equal(labels, want) {
		t.Fatalf("labels = %q\nwant %q", labels, want)
	}
}

func dashboardApps() []Application {
	return []Application{
		{ID: "a-overdue", Status: "interviewing", Process: Process{
			NextStepDate: NewDate(2026, 10, 5), // overdue
			DateApplied:  NewDate(2026, 9, 1),
			Interviews:   []Interview{{Date: NewDateTime(2026, 10, 9, 10, 0)}}, // upcoming
		}},
		{ID: "b-followup-done", Status: "applied", Process: Process{
			FollowUpDate:    NewDate(2026, 10, 1),
			LastContactDate: NewDate(2026, 10, 2), // contacted after follow-up date → not overdue
			DateApplied:     NewDate(2026, 9, 20),
		}},
		{ID: "c-stale", Status: "applied", Process: Process{
			DateApplied: NewDate(2026, 9, 1), // 36 days ago, nothing scheduled
		}},
		{ID: "d-closed", Status: "rejected", Process: Process{
			NextStepDate: NewDate(2026, 10, 1), // closed → ignored
			DateApplied:  NewDate(2026, 8, 1),
			Interviews:   []Interview{{Date: NewDateTime(2026, 8, 20, 10, 0)}},
		}},
		{ID: "e-accepted", Status: "accepted", Compensation: Compensation{StartDate: NewDate(2026, 10, 19)},
			Process: Process{DateApplied: NewDate(2026, 8, 1), OfferDate: NewDate(2026, 9, 25)}},
		{ID: "f-far", Status: "screening", Process: Process{
			ScreeningDate: NewDateTime(2026, 12, 1, 9, 0), // beyond the horizon, but scheduled → not stale
			DateApplied:   NewDate(2026, 8, 1),
		}},
		{ID: "g-wishlist", Status: "wishlist"},
		{ID: "h-unknown", Status: "on-hold", Process: Process{FollowUpDate: NewDate(2026, 10, 7)}},
	}
}

func TestDashboard(t *testing.T) {
	cfg := DefaultConfig()
	apps := dashboardApps()
	d := BuildDashboard(apps, &cfg, today)

	var overdue []string
	for _, e := range d.Overdue {
		overdue = append(overdue, e.App.ID+"/"+string(e.Kind))
	}
	if want := []string{"a-overdue/next-step"}; !slices.Equal(overdue, want) {
		t.Errorf("overdue = %v, want %v", overdue, want)
	}

	var upcoming []string
	for _, day := range d.Upcoming {
		for _, e := range day.Events {
			upcoming = append(upcoming, day.Day.String()+" "+e.App.ID+"/"+string(e.Kind))
		}
	}
	want := []string{
		"2026-10-07 h-unknown/follow-up", // today counts; unknown status counts as active
		"2026-10-09 a-overdue/interview",
		"2026-10-19 e-accepted/start", // start dates show even for closed applications
	}
	if !slices.Equal(upcoming, want) {
		t.Errorf("upcoming = %v\nwant %v", upcoming, want)
	}

	var stale []string
	for _, s := range d.Stale {
		stale = append(stale, s.App.ID)
	}
	if want := []string{"c-stale"}; !slices.Equal(stale, want) {
		t.Errorf("stale = %v, want %v", stale, want)
	}
	if d.Stale[0].Days != 36 {
		t.Errorf("stale days = %d", d.Stale[0].Days)
	}

	if d.Active != 6 || d.Total != 8 {
		t.Errorf("active=%d total=%d", d.Active, d.Total)
	}
	last := d.Pipeline[len(d.Pipeline)-1]
	if last.Status.ID != "on-hold" || last.Known || last.Count != 1 {
		t.Errorf("unknown status not appended to pipeline: %+v", last)
	}
	if d.Pipeline[1].Status.ID != "applied" || d.Pipeline[1].Count != 2 {
		t.Errorf("applied count: %+v", d.Pipeline[1])
	}
	// cumulative: applied a,b,c,d,e,f; screening a,d,e,f; interview a,d,e; offer e
	if want := (Funnel{Applied: 6, Screening: 4, Interview: 3, Offer: 1}); d.Funnel != want {
		t.Errorf("funnel = %+v, want %+v", d.Funnel, want)
	}
}

func TestUpcomingUnlimited(t *testing.T) {
	cfg := DefaultConfig()
	days := Upcoming(dashboardApps(), &cfg, today, -1)
	if last := days[len(days)-1]; last.Day != NewDate(2026, 12, 1) {
		t.Fatalf("last day = %v", last.Day)
	}
}

func TestSalaryString(t *testing.T) {
	tests := []struct {
		in   SalaryRange
		want string
	}{
		{SalaryRange{Min: 80000, Max: 95000, Currency: "GBP", Period: "year"}, "£80,000–95,000 / year"},
		{SalaryRange{Min: 92000, Currency: "GBP", Period: "year"}, "£92,000+ / year"},
		{SalaryRange{Max: 600, Currency: "CHF", Period: "day"}, "up to 600 CHF / day"},
		{SalaryRange{Min: 500, Max: 500, Currency: "EUR", Period: "day", Note: "DOE"}, "€500 / day (DOE)"},
		{SalaryRange{Note: "Competitive"}, "Competitive"},
		{SalaryRange{Currency: "GBP", Period: "year"}, ""},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("%+v: got %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeText(t *testing.T) {
	tests := map[string]string{
		"":                         "",
		"  one line  ":             "  one line",
		"\r\n- a  \r\n- b\r\n\r\n": "- a\n- b\n",
		"x\n\n\ny":                 "x\n\n\ny\n",
	}
	for in, want := range tests {
		if got := NormalizeText(in); got != want {
			t.Errorf("NormalizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConfigFillsDefaults(t *testing.T) {
	cfg, err := ParseConfig([]byte("schemaVersion: 1\nstatuses:\n  - { id: open }\n  - { id: done, closed: true }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StaleAfterDays != 14 || len(cfg.Channels) == 0 || len(cfg.Priorities) != 3 {
		t.Fatalf("defaults not filled: %+v", cfg)
	}
	if cfg.StatusLabel("open") != "open" || cfg.IsActive("done") || !cfg.IsActive("whatever") {
		t.Fatal("status lookups")
	}
	if cfg.DefaultStatus() != "open" {
		t.Fatalf("DefaultStatus = %q", cfg.DefaultStatus())
	}
}

func TestApplyStatusSetsDateApplied(t *testing.T) {
	a := Application{Status: "wishlist"}
	a.ApplyStatus("applied", today)
	if a.Process.DateApplied != today {
		t.Fatal("date applied not set")
	}
	a.ApplyStatus("screening", today.AddDays(3))
	a.ApplyStatus("applied", today.AddDays(5))
	if a.Process.DateApplied != today {
		t.Fatal("date applied overwritten")
	}
}
