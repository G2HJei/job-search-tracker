package views

import (
	"maps"
	"net/url"
	"slices"
	"strconv"

	"github.com/a-h/templ"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/views/components"
)

func appURL(d DetailData) string { return "/applications/" + d.Base.App.ID }

func oobAttr(oob bool) templ.Attributes {
	if !oob {
		return nil
	}
	return templ.Attributes{"hx-swap-oob": "true"}
}

func priorityColor(cfg model.Config, id string) string {
	p, _ := cfg.Priority(id)
	return p.Color
}

func contains(list []string, s string) bool { return s != "" && slices.Contains(list, s) }

func sortedValues(m map[string]string) []string {
	return slices.Sorted(maps.Values(m))
}

func queryEscape(s string) string { return url.QueryEscape(s) }

// hostOf returns the host of a URL for compact display.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

// isOverdue reports whether an active application's next step date has passed.
func isOverdue(cfg model.Config, a model.Application, today model.Date) bool {
	d := a.Process.NextStepDate
	return !d.IsZero() && d.Before(today) && cfg.IsActive(a.Status)
}

func companyFormAction(d CompanyFormData) string {
	if d.New {
		return "/companies"
	}
	return "/companies/" + d.Company.ID
}

type keyDate struct {
	Label, Value, Note string
	Overdue            bool
}

// keyDates picks the dates worth showing in the application header.
func keyDates(d DetailData) []keyDate {
	a := d.Base.App
	today := d.Base.Today
	active := d.Base.Config.IsActive(a.Status)
	p := a.Process
	var out []keyDate
	add := func(label string, v model.Date, display, note string, actionable bool) {
		if v.IsZero() {
			return
		}
		out = append(out, keyDate{label, display, note, actionable && active && v.Before(today)})
	}
	add("Applied", p.DateApplied, p.DateApplied.Display(), "", false)
	add("Last contact", p.LastContactDate, p.LastContactDate.Display(), components.Relative(p.LastContactDate, today), false)
	add("Next step", p.NextStepDate, p.NextStepDate.Display(), p.NextStep, true)
	for i, iv := range p.Interviews {
		if !iv.Date.IsZero() && !iv.Date.Date().Before(today) {
			add("Next interview", iv.Date.Date(), iv.Date.Display(), "Interview "+strconv.Itoa(i+1)+" "+iv.Type, false)
			break
		}
	}
	add("Take-home due", p.TakeHome.Deadline.Date(), p.TakeHome.Deadline.Display(), "", true)
	add("Offer response by", p.OfferResponseDeadline, p.OfferResponseDeadline.Display(), "", true)
	add("Start date", a.Compensation.StartDate, a.Compensation.StartDate.Display(), "", false)
	return out
}
