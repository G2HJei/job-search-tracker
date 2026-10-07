package model

import (
	"cmp"
	"fmt"
	"slices"
)

// EventKind identifies which date field an event comes from.
type EventKind string

const (
	KindNextStep      EventKind = "next-step"
	KindFollowUp      EventKind = "follow-up"
	KindScreening     EventKind = "screening"
	KindInterview     EventKind = "interview"
	KindTakeHome      EventKind = "take-home"
	KindOffer         EventKind = "offer"
	KindOfferResponse EventKind = "offer-response"
	KindStart         EventKind = "start"
)

// Actionable kinds are things you have to do; they can become overdue.
func (k EventKind) Actionable() bool {
	switch k {
	case KindNextStep, KindFollowUp, KindTakeHome, KindOfferResponse:
		return true
	}
	return false
}

// Highlight reports whether the event is a meeting with the company.
func (k EventKind) Highlight() bool { return k == KindInterview || k == KindScreening }

// Event is one dated item on an application.
type Event struct {
	When  DateTime
	Kind  EventKind
	AppID string
	Label string
	N     int // interview number (1-based) for KindInterview
}

// Date returns the day of the event.
func (e Event) Date() Date { return e.When.Date() }

// Events returns every dated item on the application. The dashboard,
// calendar and ICS export are all built on it.
func Events(a Application) []Event {
	var out []Event
	add := func(when DateTime, kind EventKind, label string, n int) {
		if !when.IsZero() {
			out = append(out, Event{When: when, Kind: kind, AppID: a.ID, Label: label, N: n})
		}
	}
	p := &a.Process
	nextStep := "Next step"
	if p.NextStep != "" {
		nextStep = "Next step: " + p.NextStep
	}
	add(DateOnly(p.NextStepDate), KindNextStep, nextStep, 0)
	add(DateOnly(p.FollowUpDate), KindFollowUp, "Follow up", 0)
	add(p.ScreeningDate, KindScreening, "Screening", 0)
	for i, iv := range p.Interviews {
		label := fmt.Sprintf("Interview %d", i+1)
		if iv.Type != "" {
			label += " · " + iv.Type
		}
		add(iv.Date, KindInterview, label, i+1)
	}
	add(p.TakeHome.Deadline, KindTakeHome, "Take-home deadline", 0)
	add(DateOnly(p.OfferDate), KindOffer, "Offer", 0)
	add(DateOnly(p.OfferResponseDeadline), KindOfferResponse, "Offer response deadline", 0)
	add(DateOnly(a.Compensation.StartDate), KindStart, "Start date", 0)
	return out
}

// AppEvent is an event together with its application.
type AppEvent struct {
	Event
	App *Application
}

// DayEvents groups events on the same day.
type DayEvents struct {
	Day    Date
	Events []AppEvent
}

// done reports whether an actionable event no longer needs doing: a
// follow-up is done once you have been in contact on or after its date.
func done(e Event, a *Application) bool {
	return e.Kind == KindFollowUp && !a.Process.LastContactDate.IsZero() &&
		!a.Process.LastContactDate.Before(e.Date())
}

// visible reports whether an event shows up in active views: events on
// active applications, plus start dates for any application.
func visible(e Event, a *Application, cfg *Config) bool {
	return e.Kind == KindStart || cfg.IsActive(a.Status)
}

func sortEvents(evs []AppEvent) {
	slices.SortStableFunc(evs, func(x, y AppEvent) int {
		return cmp.Or(x.When.Compare(y.When), cmp.Compare(x.App.ID, y.App.ID))
	})
}

// Overdue returns actionable events before today on active applications.
func Overdue(apps []Application, cfg *Config, today Date) []AppEvent {
	var out []AppEvent
	for i := range apps {
		a := &apps[i]
		if !cfg.IsActive(a.Status) {
			continue
		}
		for _, e := range Events(*a) {
			if e.Kind.Actionable() && e.Date().Before(today) && !done(e, a) {
				out = append(out, AppEvent{e, a})
			}
		}
	}
	sortEvents(out)
	return out
}

// Upcoming returns visible events from today up to and including
// today+days, grouped by day. days < 0 means no limit.
func Upcoming(apps []Application, cfg *Config, today Date, days int) []DayEvents {
	var evs []AppEvent
	for _, e := range Future(apps, cfg, today) {
		if days < 0 || !e.Date().After(today.AddDays(days)) {
			evs = append(evs, e)
		}
	}
	return GroupByDay(evs)
}

// Future returns visible events on or after today, sorted.
func Future(apps []Application, cfg *Config, today Date) []AppEvent {
	var out []AppEvent
	for i := range apps {
		a := &apps[i]
		for _, e := range Events(*a) {
			if visible(e, a, cfg) && !e.Date().Before(today) {
				out = append(out, AppEvent{e, a})
			}
		}
	}
	sortEvents(out)
	return out
}

// GroupByDay groups sorted events by day.
func GroupByDay(evs []AppEvent) []DayEvents {
	var out []DayEvents
	for _, e := range evs {
		if n := len(out); n > 0 && out[n-1].Day.Equal(e.Date()) {
			out[n-1].Events = append(out[n-1].Events, e)
			continue
		}
		out = append(out, DayEvents{Day: e.Date(), Events: []AppEvent{e}})
	}
	return out
}
