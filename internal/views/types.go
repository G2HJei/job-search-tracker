// Package views renders full pages and page-level partials.
package views

import (
	"net/url"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/views/sections"
)

// htmxConfig makes htmx swap 422 (validation errors) and 409 (conflicts)
// responses, keeps injected inline styles and eval off for the CSP, and
// always reloads pages on back/forward so data is never stale.
const htmxConfig = `{"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"422","swap":true},{"code":"409","swap":true},{"code":"[45]..","swap":false,"error":true}],"includeIndicatorStyles":false,"allowEval":false,"historyCacheSize":0,"refreshOnHistoryMiss":true}`

// Page holds what the layout needs.
type Page struct {
	Title      string
	Nav        string // active nav item
	Version    string // appended to asset URLs to bust caches
	Query      string // global search box value
	Wide       bool   // use the full window width
	Board      bool   // load SortableJS
	LoadErrors int
	Warnings   []string
}

func (p Page) asset(path string) string {
	return "/static/" + path + "?v=" + url.QueryEscape(p.Version)
}

// Row is an application with its company name, for tables and cards.
type Row struct {
	App          model.Application
	Company      string
	CompanyKnown bool
}

// DashboardData feeds the dashboard.
type DashboardData struct {
	model.Dashboard
	Config model.Config
	Names  map[string]string // company id → name
	Titles map[string]string // application id → "Company · Title"
}

// ListData feeds the applications table.
type ListData struct {
	Config   model.Config
	Today    model.Date
	Rows     []Row
	Total    int // applications before filtering
	Q        string
	Status   string
	Priority string
	Closed   bool
	Sort     string
	Dir      string
}

// Query returns the current filters as URL values.
func (l ListData) Query() url.Values {
	v := url.Values{}
	set := func(k, val string) {
		if val != "" {
			v.Set(k, val)
		}
	}
	set("q", l.Q)
	set("status", l.Status)
	set("priority", l.Priority)
	if l.Closed {
		v.Set("closed", "1")
	}
	set("sort", l.Sort)
	set("dir", l.Dir)
	return v
}

// SortURL returns the list URL sorted by col, toggling the direction if it
// is already the sort column.
func (l ListData) SortURL(col string) string {
	v := l.Query()
	dir := "asc"
	if l.Sort == col && l.Dir != "desc" {
		dir = "desc"
	}
	v.Set("sort", col)
	v.Set("dir", dir)
	return "/applications?" + v.Encode()
}

// SortMark shows ▲/▼ on the active sort column.
func (l ListData) SortMark(col string) string {
	if l.Sort != col {
		return ""
	}
	if l.Dir == "desc" {
		return " ▼"
	}
	return " ▲"
}

// BoardColumn is one kanban column.
type BoardColumn struct {
	Status model.Status
	Known  bool
	Rows   []Row
}

// BoardData feeds the kanban board.
type BoardData struct {
	Config  model.Config
	Today   model.Date
	Columns []BoardColumn
}

// CalendarData feeds the agenda.
type CalendarData struct {
	Config  model.Config
	Today   model.Date
	Overdue []model.AppEvent
	Days    []model.DayEvents
	Titles  map[string]string // application id → "Company · Title"
	ICSURL  string
}

// DetailData feeds the application page.
type DetailData struct {
	Base        sections.Data // stored data, used by every section card
	EditName    string        // section shown in edit mode (no-JS flow)
	EditData    sections.Data // data for the section in edit mode
	Warnings    model.FieldErrors
	PostingText string // contents of a Markdown posting copy
}

// NewAppData feeds the quick-add form.
type NewAppData struct {
	Config     model.Config
	Companies  []model.Company
	Company    string
	Title      string
	PostingURL string
	Priority   string
	Status     string
	Errors     model.FieldErrors
}

// CompanyRow is a company with its application counts.
type CompanyRow struct {
	Company model.Company
	Active  int
	Total   int
}

// CompanyData feeds the company page.
type CompanyData struct {
	Company model.Company
	Config  model.Config
	Today   model.Date
	Rows    []Row
}

// CompanyFormData feeds the new/edit company form.
type CompanyFormData struct {
	Company    model.Company
	Config     model.Config
	Industries []string
	Errors     model.FieldErrors
	Error      string
	Rev        string
	New        bool
}

// CVItem is a file in the CV library and where it was used.
type CVItem struct {
	Name    string
	Size    int64
	ModTime time.Time
	UsedIn  []Row
}

// CVData feeds the CV library page.
type CVData struct {
	Config model.Config
	Files  []CVItem
	Error  string
}

// LoadError is a file that could not be loaded.
type LoadError struct {
	Path, Err string
}

// SettingsData feeds the settings page.
type SettingsData struct {
	Dir          string
	Version      string
	ConfigYAML   string
	LoadErrors   []LoadError
	Warnings     []string
	Applications int
	Companies    int
}
