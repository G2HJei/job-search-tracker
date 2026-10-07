# Job Search Tracker — Implementation Plan

Stack: **Go + templ + htmx, shipped as a single binary.**
A local web app for tracking job applications. Data lives in human-readable YAML files and there is no database.

---

## 1. Goals and non-goals

**Goals**
- Track every data point listed in [§5 Field mapping](#5-field-mapping-spec--yaml--go--ui), using the same grouping.
- Local only: one executable (`tracker.exe`) that serves a web UI on `127.0.0.1` and stores data next to itself.
- Plain-file storage that you can read, edit, diff, back up and put in git outside the app.
- Fast daily workflow: see what's overdue or coming up, update status in one click, add an application in under 30 seconds.

**Non-goals (v1)**
- Multi-user use, authentication, cloud sync or hosting.
- Mobile-first layout. It should be usable on a phone, but desktop is the target.
- Scraping job boards or automating applications.

---

## 2. Tech stack

| Concern | Choice | Notes |
|---|---|---|
| Language | **Go 1.27** (current stable, Aug 2026) | Uses `os.Root` (1.24+), `go get -tool` (1.24+), `http.CrossOriginProtection` (1.25+) |
| HTTP | Standard library `net/http` + `ServeMux` | Method and path patterns (`GET /applications/{id}`) cover routing. No router library needed |
| Templates | **templ** (`github.com/a-h/templ`) | Type-safe components compiled to Go. Generated `*_templ.go` files are committed so `go build` works without templ installed |
| Interactivity | **htmx 2.x** (vendored) | 2.x still holds the npm `latest` tag and is supported indefinitely. htmx 4.0 (Aug 2026) is on `next`. Revisit when 4.x becomes `latest` |
| Drag and drop | SortableJS (vendored) | Kanban board only |
| CSS | Pico CSS v2 (vendored) + a small `app.css` | Good default form styling, automatic dark mode |
| Small client logic | Optional: Alpine.js 3 | Only if htmx and a few lines of plain JS aren't enough. The standard build needs CSP `unsafe-eval` |
| Storage | YAML via **`go.yaml.in/yaml/v3`** | Maintained continuation of `gopkg.in/yaml.v3` (archived Apr 2025) under the YAML org |
| Markdown rendering | `github.com/yuin/goldmark` | For notes, lessons learned, requirements and Q&A. Raw HTML and dangerous URLs are dropped by default |
| Logging | `log/slog` | |
| Dev tools (`go tool`) | `templ`, `air` (live reload), `task` (task runner) | Installed as tool dependencies in `go.mod`, so nothing is installed globally. Also avoids a clash: `air` on this machine's PATH is a JetBrains Toolbox script, so always run `go tool air` |

All frontend assets are **vendored into `web/static/vendor/` and embedded**, with no CDN, so the app works offline.
Record the pinned versions in `web/static/vendor/VERSIONS.md` and keep each library's license header. The libraries are 0BSD or MIT, which is compatible with the repo's GPL-3.0.

---

## 3. Architecture

```
browser ──HTTP──▶ web (handlers, form binding, middleware)
                   │            │
                   ▼            ▼
                views (templ)  model (types, validation, events; no I/O)
                   ▲            ▲
                   └──── store (YAML files ⇄ in-memory index) ──▶ data/
```

- **model**: pure domain types and logic: structs, validation, date types, event derivation for the dashboard and calendar. No file or HTTP code. Easy to unit test.
- **store**: the only package that touches `data/`. Loads everything into memory at startup, serves reads from memory, and writes to disk atomically before updating memory. Uses one `sync.RWMutex`.
- **web**: routes, handlers, form parsing, htmx-aware rendering (full page vs partial), and security middleware.
- **views**: templ components. Pages, plus one *view* and one *edit* partial per field group.

Data volume is small (hundreds of applications at most), so keeping everything in memory is simple and fast. Sorting, filtering and search are plain Go over slices.

### Project layout

```
job-search-tracker/
├── cmd/tracker/main.go            # flags, wiring, graceful shutdown, --open browser
├── internal/
│   ├── model/
│   │   ├── application.go         # Application + group structs
│   │   ├── company.go
│   │   ├── contact.go
│   │   ├── dates.go               # Date, DateTime (YAML + form encoding)
│   │   ├── money.go               # SalaryRange
│   │   ├── lookups.go             # config lists (statuses, priorities, …) + defaults
│   │   ├── events.go              # derive dated events → dashboard / calendar / ICS
│   │   └── validate.go
│   ├── store/
│   │   ├── store.go               # Open/Load, index, RWMutex, load errors
│   │   ├── applications.go        # CRUD, trash
│   │   ├── companies.go
│   │   ├── files.go               # attachments via os.Root, filename sanitising
│   │   ├── atomic.go              # writeFileAtomic (+ Windows rename retry)
│   │   └── slug.go
│   ├── web/
│   │   ├── server.go              # mux + middleware chain
│   │   ├── routes.go
│   │   ├── handlers_dashboard.go
│   │   ├── handlers_applications.go
│   │   ├── handlers_companies.go
│   │   ├── handlers_files.go
│   │   ├── handlers_export.go
│   │   ├── forms.go               # parse helpers + per-section binders
│   │   └── render.go              # render(w, r, page, partial), HX-Request detection
│   ├── views/
│   │   ├── layout.templ
│   │   ├── components/            # pill, rating(1–5), date inputs, salary input, contact rows, markdown
│   │   ├── dashboard.templ
│   │   ├── applications_list.templ
│   │   ├── board.templ
│   │   ├── calendar.templ
│   │   ├── application_detail.templ
│   │   ├── sections/              # company, job, contacts, process, compensation, other (view + edit)
│   │   ├── companies.templ
│   │   └── settings.templ
│   └── demo/
│       ├── demo.go                # //go:embed sample → used by --demo and by tests
│       └── sample/                # fake data dir: config.yaml, companies/, applications/
├── web/
│   ├── embed.go                   # //go:embed static   → embed.FS
│   └── static/
│       ├── css/app.css
│       ├── js/app.js              # Sortable wiring, small helpers
│       └── vendor/                # htmx.min.js, Sortable.min.js, pico.min.css, VERSIONS.md
├── data/                          # YOUR real data. Git-ignored, never committed
├── Taskfile.yml
├── .air.toml
├── .gitignore
├── go.mod / go.sum
├── PLAN.md
└── README.md
```

---

## 4. File storage design

### Data directory

```
data/
├── config.yaml                          # dropdown lists + settings, created with defaults on first run
├── companies/
│   └── acme.yaml                        # Company group, shared by all applications to that company
├── applications/
│   └── 2026-10-07-acme-backend-engineer/
│       ├── application.yaml             # everything except the Company group
│       └── files/                       # posting copy, cover letter, take-home submission, …
├── cv/                                  # your CV library: cv-backend-v3.pdf, cv-lead-v1.pdf, …
└── .trash/                              # deleted applications are moved here, never hard-deleted
```

How the data directory is resolved: `--data` flag → `JST_DATA_DIR` env var → `data/` next to the executable.
Dev tasks always pass `--data ./data` explicitly, because `go run` and air build into temp folders.

### Rules
- **IDs**: `YYYY-MM-DD-<company-slug>-<title-slug>`, generated once at creation and never changed (renaming a job title does not rename the folder). On collision, append `-2`, `-3`, …. Company IDs are the slugified name, also immutable.
- **Format**: YAML with `schemaVersion: 1` at the top of each file, so files can be migrated later. YAML is used instead of JSON because multi-line notes stay readable (`|` blocks) and hand edits are easy.
- **Atomic writes**: write `.tmp-*` in the same folder → `Sync()` → `Close()` → `os.Rename` over the target. On Windows, `os.Rename` replaces the target, but antivirus or the search indexer can briefly lock files, so retry about 5 times with short backoff.
- **Load errors**: a file that fails to parse is skipped and listed in a banner ("2 files could not be loaded: …"). The app **never overwrites a file it failed to parse.**
- **Unknown values**: if a status, channel or other value isn't in `config.yaml` anymore, keep it and show it as-is with a warning style. Never drop data.
- **Concurrent edits** (M5): each edit form carries a hidden `rev` (the record's `updatedAt`). On save, a mismatch returns 409 and shows "changed elsewhere, reload". Before writing, the store checks the file's mtime to detect external edits. A "Reload from disk" button covers manual edits; `fsnotify` can be added later.
- **Attachments**: all file access goes through `os.OpenRoot(dataDir)`, which blocks path traversal. Filenames are reduced with `filepath.Base` and stripped to `[A-Za-z0-9._-]`. Uploads are capped at 25 MB (`http.MaxBytesReader`).
- **Backups**: recommend `git init` inside `data/` (separate from the app repo). M5 adds a "Download backup (.zip)" button.

### `config.yaml` (defaults written on first run)

```yaml
schemaVersion: 1
defaultCurrency: GBP          # change to your currency
staleAfterDays: 14            # active applications with no contact for this long are flagged
upcomingDays: 14              # dashboard horizon
priorities:                   # id is stored in application files; label/color are display-only
  - { id: high,   label: High,   color: red }
  - { id: medium, label: Medium, color: amber }
  - { id: low,    label: Low,    color: grey }
statuses:                     # order = kanban column order; closed = hidden from "active" views
  - { id: wishlist,     label: Wishlist }
  - { id: applied,      label: Applied }
  - { id: screening,    label: Screening }
  - { id: interviewing, label: Interviewing }
  - { id: offer,        label: Offer }
  - { id: accepted,     label: Accepted,    closed: true }
  - { id: rejected,     label: Rejected,    closed: true }
  - { id: withdrawn,    label: Withdrawn,   closed: true }
  - { id: ghosted,      label: No response, closed: true }
channels:       [Company website, LinkedIn, Referral, Recruiter (inbound), Recruitment agency, Job board, Other]
contractTypes:  [Permanent, Fixed-term, Contractor, Freelance, Part-time, Internship]
decisions:      [Pending, Offer accepted, Offer declined, Rejected by company, Withdrew]
companySizes:   ["1-10", "11-50", "51-200", "201-1000", "1001-5000", "5000+"]
interviewTypes: [Phone screen, Technical, System design, Behavioural, Case study, Panel, Final, Other]
salaryPeriods:  [year, month, day, hour]
```

### Example `application.yaml`

```yaml
schemaVersion: 1
id: 2026-10-07-acme-backend-engineer
createdAt: 2026-10-07T15:20:00+01:00
updatedAt: 2026-10-09T09:02:11+01:00
priority: high
status: interviewing
companyId: acme
job:
  title: Senior Backend Engineer
  postingUrl: https://acme.example/careers/123
  postingCopy: posting.pdf
  jobId: R-12345
  department: Payments Platform
  location: London (hybrid, 2 days)
  keyRequirements: |
    - 5+ years Go or Java
    - Kubernetes, Postgres
  fit: 4
  interest: 5
  channel: Referral
  cvVersion: cv-backend-v3.pdf
  extraMaterials:
    - cover-letter.pdf
    - https://github.com/me/portfolio
  advertisedSalary: { min: 80000, max: 95000, currency: GBP, period: year }
contacts:
  referrer: { name: Jane Smith, role: Staff Engineer, email: jane@example.com }
  recruiters:
    - name: Tom Lee
      role: Talent Partner
      email: tom@acme.example
      linkedin: https://linkedin.com/in/tomlee
  hiringManager: { name: Priya Patel, role: Engineering Manager }
  interviewers:
    - { name: Alex Kim, role: Senior Engineer }
process:
  nextStep: Technical interview (live coding)
  nextStepDate: 2026-10-14
  lastContactDate: 2026-10-09
  dateApplied: 2026-10-07
  screeningDate: 2026-10-09T10:00
  interviews:
    - date: 2026-10-14T14:00
      type: Technical
      interviewers: [Alex Kim]
  followUpDate: 2026-10-16
  qa:
    - round: 0
      question: Why are you leaving your current role?
      answer: |
        Looking for larger-scale distributed systems work…
  decision: Pending
compensation:
  asked: { min: 92000, currency: GBP, period: year }
  contractType: Permanent
  noticePeriod: 1 month
other:
  notes: |
    Strong engineering blog; ask about on-call.
```

---

## 5. Field mapping (spec → YAML → Go → UI)

Every field in the spec maps to exactly one storage location. Rows marked † are the only modeling choices beyond the spec. Each is explained in [§13](#13-decisions-made-change-if-you-disagree).

| Group | Spec field | YAML key | Go type | UI control |
|---|---|---|---|---|
| — | Priority | `priority` | `string` (config id) | select / colored pill, inline-editable |
| — | Status | `status` | `string` (config id) | select / pill, inline-editable, kanban column |
| Company | Company | `companies/<id>.yaml: name` (+ `companyId` on application) | `Company.Name` | text with autocomplete over existing companies, or create inline |
| Company | Industry | `industry` | `string` | text with datalist of existing values |
| Company | Company size | `size` | `string` (config) | select |
| Company | HQ location | `hq` | `string` | text |
| Company | Website | `website` | `string` (http/https URL) | url input, rendered as link |
| Job | Job title | `job.title` | `string` (required) | text |
| Job | Job posting link | `job.postingUrl` | `string` (URL) | url input |
| Job | Saved posting copy | `job.postingCopy` | `string` (file in `files/`) | upload (PDF/HTML/MHTML) **or** paste text → saved as `posting.md` |
| Job | Job ID | `job.jobId` | `string` | text |
| Job | Department / team | `job.department` | `string` | text |
| Job | Location | `job.location` | `string` | text |
| Job | Key requirements | `job.keyRequirements` | `string` (Markdown) | textarea |
| Job | Fit (1–5) | `job.fit` | `int` (0 = unset) | 1–5 radio "stars" |
| Job | Interest level (1–5) | `job.interest` | `int` (0 = unset) | 1–5 radio "stars" |
| Job | Application channel | `job.channel` | `string` (config) | select |
| Job | CV version | `job.cvVersion` | `string` | select from files in `data/cv/` + free text |
| Job | Extra materials sent | `job.extraMaterials` | `[]string` (file names or URLs) | repeatable rows + upload |
| Job | Advertised salary range | `job.advertisedSalary` | `SalaryRange` | min / max / currency / period / note |
| Contacts | Referrer | `contacts.referrer` | `*Contact` | contact row |
| Contacts | Recruiter contacts | `contacts.recruiters` | `[]Contact` | repeatable contact rows |
| Contacts | Hiring manager | `contacts.hiringManager` | `*Contact` | contact row |
| Contacts | Interviewers | `contacts.interviewers` | `[]Contact` | repeatable contact rows |
| Process | Next step | `process.nextStep` | `string` | text |
| Process | Next step date | `process.nextStepDate` | `Date` | date |
| Process | Last contact date | `process.lastContactDate` | `Date` | date + "Today" button |
| Process | Date applied | `process.dateApplied` | `Date` | date (auto-set when status → applied, editable) |
| Process | Screening date | `process.screeningDate` | `DateTime` | datetime-local |
| Process | Interview 1 / 2 / 3 date † | `process.interviews[n].date` | `[]Interview` | rows labelled "Interview 1, 2, 3" + "Add round" |
| Process | Follow-up date | `process.followUpDate` | `Date` | date |
| Process | Offer date | `process.offerDate` | `Date` | date |
| Process | Questions asked / answers † | `process.qa` | `[]QA` (optional round link) | repeatable Q/A rows |
| Process | Take-home task details | `process.takeHome.details` | `string` (Markdown) | textarea |
| Process | Take-home deadline | `process.takeHome.deadline` | `DateTime` | datetime-local |
| Process | Decision | `process.decision` | `string` (config) | select |
| Process | Rejection reason / feedback | `process.rejectionFeedback` | `string` (Markdown) | textarea |
| Process | Offer response deadline | `process.offerResponseDeadline` | `Date` | date |
| Compensation | Asked salary † | `compensation.asked` | `SalaryRange` | min (+ optional max) / currency / period |
| Compensation | Offered salary † | `compensation.offered` | `SalaryRange` | same |
| Compensation | Contract type | `compensation.contractType` | `string` (config) | select |
| Compensation | Bonus / commission | `compensation.bonus` | `string` | text |
| Compensation | Equity | `compensation.equity` | `string` | text |
| Compensation | Benefits | `compensation.benefits` | `string` (Markdown) | textarea |
| Compensation | Notice period / start date † | `compensation.noticePeriod` + `compensation.startDate` | `string` + `Date` | text + date |
| Other | Lessons learned | `other.lessonsLearned` | `string` (Markdown) | textarea |
| Other | Notes | `other.notes` | `string` (Markdown) | textarea |

### Go types (sketch)

```go
package model

type Application struct {
	SchemaVersion int       `yaml:"schemaVersion"`
	ID            string    `yaml:"id"`
	CreatedAt     time.Time `yaml:"createdAt"`
	UpdatedAt     time.Time `yaml:"updatedAt"`

	Priority  string `yaml:"priority"`  // config.priorities[].id
	Status    string `yaml:"status"`    // config.statuses[].id
	CompanyID string `yaml:"companyId"` // companies/<id>.yaml

	Job          Job          `yaml:"job"`
	Contacts     Contacts     `yaml:"contacts,omitempty"`
	Process      Process      `yaml:"process,omitempty"`
	Compensation Compensation `yaml:"compensation,omitempty"`
	Other        Other        `yaml:"other,omitempty"`
}

type Company struct {
	SchemaVersion int    `yaml:"schemaVersion"`
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Industry      string `yaml:"industry,omitempty"`
	Size          string `yaml:"size,omitempty"`
	HQ            string `yaml:"hq,omitempty"`
	Website       string `yaml:"website,omitempty"`
	Notes         string `yaml:"notes,omitempty"`
}

type Job struct {
	Title            string      `yaml:"title"`
	PostingURL       string      `yaml:"postingUrl,omitempty"`
	PostingCopy      string      `yaml:"postingCopy,omitempty"`
	JobID            string      `yaml:"jobId,omitempty"`
	Department       string      `yaml:"department,omitempty"`
	Location         string      `yaml:"location,omitempty"`
	KeyRequirements  string      `yaml:"keyRequirements,omitempty"`
	Fit              int         `yaml:"fit,omitempty"`
	Interest         int         `yaml:"interest,omitempty"`
	Channel          string      `yaml:"channel,omitempty"`
	CVVersion        string      `yaml:"cvVersion,omitempty"`
	ExtraMaterials   []string    `yaml:"extraMaterials,omitempty"`
	AdvertisedSalary SalaryRange `yaml:"advertisedSalary,omitempty"`
}

type Contact struct {
	Name     string `yaml:"name"`
	Role     string `yaml:"role,omitempty"`
	Email    string `yaml:"email,omitempty"`
	Phone    string `yaml:"phone,omitempty"`
	LinkedIn string `yaml:"linkedin,omitempty"`
	Notes    string `yaml:"notes,omitempty"`
}

type Contacts struct {
	Referrer      *Contact  `yaml:"referrer,omitempty"`
	Recruiters    []Contact `yaml:"recruiters,omitempty"`
	HiringManager *Contact  `yaml:"hiringManager,omitempty"`
	Interviewers  []Contact `yaml:"interviewers,omitempty"`
}

type Process struct {
	NextStep              string      `yaml:"nextStep,omitempty"`
	NextStepDate          Date        `yaml:"nextStepDate,omitempty"`
	LastContactDate       Date        `yaml:"lastContactDate,omitempty"`
	DateApplied           Date        `yaml:"dateApplied,omitempty"`
	ScreeningDate         DateTime    `yaml:"screeningDate,omitempty"`
	Interviews            []Interview `yaml:"interviews,omitempty"`
	FollowUpDate          Date        `yaml:"followUpDate,omitempty"`
	OfferDate             Date        `yaml:"offerDate,omitempty"`
	QA                    []QA        `yaml:"qa,omitempty"`
	TakeHome              TakeHome    `yaml:"takeHome,omitempty"`
	Decision              string      `yaml:"decision,omitempty"`
	RejectionFeedback     string      `yaml:"rejectionFeedback,omitempty"`
	OfferResponseDeadline Date        `yaml:"offerResponseDeadline,omitempty"`
}

type Interview struct {
	Date         DateTime `yaml:"date,omitempty"`
	Type         string   `yaml:"type,omitempty"`
	Interviewers []string `yaml:"interviewers,omitempty"` // names from contacts.interviewers
	Notes        string   `yaml:"notes,omitempty"`
}

type QA struct {
	Round    int    `yaml:"round,omitempty"` // 0 = general/screening, n = interview n
	Question string `yaml:"question"`
	Answer   string `yaml:"answer,omitempty"`
}

type TakeHome struct {
	Details  string   `yaml:"details,omitempty"`
	Deadline DateTime `yaml:"deadline,omitempty"`
}

type Compensation struct {
	Asked        SalaryRange `yaml:"asked,omitempty"`
	Offered      SalaryRange `yaml:"offered,omitempty"`
	ContractType string      `yaml:"contractType,omitempty"`
	Bonus        string      `yaml:"bonus,omitempty"`
	Equity       string      `yaml:"equity,omitempty"`
	Benefits     string      `yaml:"benefits,omitempty"`
	NoticePeriod string      `yaml:"noticePeriod,omitempty"`
	StartDate    Date        `yaml:"startDate,omitempty"`
}

type Other struct {
	LessonsLearned string `yaml:"lessonsLearned,omitempty"`
	Notes          string `yaml:"notes,omitempty"`
}

type SalaryRange struct {
	Min      int    `yaml:"min,omitempty"`
	Max      int    `yaml:"max,omitempty"`
	Currency string `yaml:"currency,omitempty"`
	Period   string `yaml:"period,omitempty"` // year | month | day | hour
	Note     string `yaml:"note,omitempty"`   // "DOE", "+10% bonus", raw text from the ad
}
```

**Date types** (`dates.go`):
- `Date` stores a calendar date with no time zone as `"2006-01-02"`. `DateTime` stores local wall-clock time as `"2006-01-02T15:04"`, which matches `<input type="datetime-local">`, and also accepts date-only values.
- Both implement `IsZero()`, so `omitempty` drops unset values. They also implement `MarshalYAML`/`UnmarshalYAML` and a `FormValue()`/`ParseForm(string)` pair for inputs.
- `MarshalYAML` should return a scalar `*yaml.Node` rather than a plain string, which the encoder would quote as `"2026-10-14"`. `UnmarshalYAML` reads `node.Value`, so quoted and unquoted forms both load.
- "Today" is computed in `time.Local`. Inject `now func() time.Time` wherever "today" matters so tests are deterministic.

**Events** (`events.go`): `func Events(a Application) []Event` returns `{When, Kind, AppID, Label}` for every date field. Kinds are next step, follow-up, screening, each interview, take-home deadline, offer, and offer response deadline. The dashboard, calendar and ICS export are all built on this one function.

**Validation** (`validate.go`) returns a `FieldErrors map[string]string`:
- Title and company are required.
- Fit and interest must be 0–5.
- URLs must be `http`/`https`.
- Salary min ≤ max.
- Dates must parse.
- A status or priority not in config produces a warning, not an error.

---

## 6. HTTP routes

Forms use GET/POST only, so pages also work without JavaScript. htmx calls the same endpoints and gets partials back when `HX-Request: true`.

| Method & path | Purpose |
|---|---|
| `GET /` | Dashboard |
| `GET /applications` | Table. Query params: `q`, `status`, `priority`, `active=1`, `sort`, `dir` |
| `GET /applications/new` | Quick-add form |
| `POST /applications` | Create → redirect (`HX-Location`) to detail |
| `GET /applications/{id}` | Detail page (all groups as section cards) |
| `GET /applications/{id}/sections/{section}` | Section **view** partial (used for Cancel) |
| `GET /applications/{id}/sections/{section}/edit` | Section **edit** partial |
| `POST /applications/{id}/sections/{section}` | Save section → returns view partial (422 + edit partial with errors on failure) |
| `POST /applications/{id}/status` | Set status (kanban drop, inline select) |
| `POST /applications/{id}/priority` | Set priority (inline select) |
| `POST /applications/{id}/touch` | "Contacted today": sets last contact date |
| `POST /applications/{id}/delete` | Move to `.trash/` |
| `POST /applications/{id}/files` | Upload attachment (multipart) |
| `POST /applications/{id}/posting-text` | Save pasted posting text as `posting.md` |
| `GET /applications/{id}/files/{name}` | Download/view attachment |
| `POST /applications/{id}/files/{name}/delete` | Remove attachment (to trash) |
| `GET /partials/row/{kind}` | Blank repeatable row: `contact`, `interview`, `qa`, `material` |
| `GET /board` | Kanban by status |
| `GET /calendar` | Upcoming and overdue events (agenda list; month grid is optional) |
| `GET /calendar.ics` | iCalendar export of all future events (subscribe from Outlook/Google) |
| `GET /companies` · `GET /companies/{id}` | Company list · company page with its applications |
| `GET /companies/new` · `POST /companies` | Create company |
| `GET /companies/{id}/edit` · `POST /companies/{id}` | Edit company |
| `GET /cv` · `POST /cv` | CV library list and upload |
| `GET /export.csv` | One row per application, all fields flattened |
| `GET /backup.zip` | Zip of the whole data directory |
| `GET /settings` · `POST /settings/reload` | Show config, data dir, load errors · reload from disk |
| `GET /static/…` | Embedded assets (served from disk with `--dev`) |
| `GET /healthz` | Used by `--open` to wait for readiness |

**htmx notes**
- htmx 2 does **not** swap 4xx/5xx responses by default. To show validation errors in place, configure it once in `layout.templ`:
  `<meta name="htmx-config" content='{"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"422","swap":true},{"code":"[45]..","swap":false,"error":true}]}'>`
- `render.go` returns the partial for htmx requests and the full layout otherwise, using the same handler for both.
- After a save, send `HX-Trigger: {"toast":"Saved"}` for a small notification.
- Repeatable rows: each row repeats the same input names (`recruiter_name`, `recruiter_email`, …), and the binder zips `r.Form["recruiter_name"]` etc. by index. "Add" uses `hx-get="/partials/row/contact?prefix=recruiter" hx-target="#recruiters" hx-swap="beforeend"`. "Remove" is a one-line JS handler that removes the row element.

---

## 7. UI screens

**Global**: top nav with Dashboard · Applications · Board · Calendar · Companies · CV · Settings, plus a primary **+ New application** button and a search box (`/` focuses it).

1. **Dashboard** (default page)
    - **Overdue**: actionable events before today on active applications (next step, follow-up, take-home deadline, offer response deadline).
    - **Upcoming**: the next `upcomingDays` of events, grouped by day, with interviews highlighted.
    - **Needs follow-up**: active applications whose latest of (last contact, date applied) is older than `staleAfterDays` and that have no future event.
    - **Pipeline**: count per status (simple CSS bars), plus applied → screening → interview → offer conversion and the number of active applications.
2. **Applications table**
    - Columns: Priority, Status, Company, Job title, Location, Fit, Interest, Next step, Next step date, Last contact, Applied.
    - Sort by clicking a header. Filters for status and priority, plus a free-text search over company, title, notes and contacts. The search uses `hx-trigger="input changed delay:300ms"` with `hx-push-url` so filtered URLs can be bookmarked.
    - "Show closed" toggle. Overdue dates shown in red.
3. **Board (kanban)**
    - Columns follow the order in `config.statuses`. Cards show company, title, priority pill and next step date.
    - Dragging a card (SortableJS) sends `POST /applications/{id}/status`. Without JS, each card has a status select instead.
4. **Application detail**
    - Header: company · job title, inline priority and status selects, posting link, and a key dates strip (applied, next step, next interview, deadlines).
    - Section cards in spec order: **Company · Job Application · Contacts · Hiring Process · Compensation · Other**. Each card has an **Edit** button that swaps the card into a form (`hx-target="closest section" hx-swap="outerHTML"`). Save and Cancel swap it back. This keeps more than 50 fields manageable without one giant form.
    - Attachments panel: posting copy, materials and links to the CV file.
    - Markdown fields are rendered with goldmark.
5. **Quick add**: company (autocomplete or new), job title, posting link, priority and status. Creates the application and opens its detail page to fill in the rest.
6. **Companies**: list with application counts, plus a company page showing its details and all applications to it.
7. **CV library**: files in `data/cv/` with upload, and where each CV was used (reverse lookup over `job.cvVersion`).
8. **Settings**: data directory path, load errors, config preview and a reload button. Lookup lists are edited in `config.yaml` directly; an in-app editor is a later option.

---

## 8. Security (local app hardening)

Local apps can still be attacked by websites you visit, through CSRF to localhost and DNS rebinding.
- Bind to **`127.0.0.1:8765`** by default. Listening on other interfaces requires an explicit `--addr`.
- Wrap the mux in **`http.NewCrossOriginProtection().Handler(...)`** (Go 1.25+). It rejects cross-origin non-GET requests using `Sec-Fetch-Site`/`Origin`.
- **Host header allowlist** (`127.0.0.1`, `localhost`, plus the configured host) to block DNS rebinding.
- templ escapes output automatically and sanitises `href` values (blocks `javascript:`). URLs are also validated as http/https on save.
- Saved HTML posting copies are served with `Content-Security-Policy: sandbox`, so scripts in a saved page can't run on the app's origin. Other non-PDF and non-image files are served with `Content-Disposition: attachment`.
- File access goes through `os.Root`. Upload size is limited and filenames are sanitised.
- M5: a strict CSP for the app itself (`default-src 'self'`). If Alpine is added, use its CSP build.

---

## 9. Build, run and dev workflow

### First-time setup (PowerShell)

```powershell
winget install GoLang.Go            # or let GoLand download the SDK (Settings → Go → GOROOT)
go version                          # expect go1.27.x

go mod init github.com/<you>/job-search-tracker
go get -tool github.com/a-h/templ/cmd/templ@latest   # also adds the templ runtime module, same version
go get -tool github.com/air-verse/air@latest
go get -tool github.com/go-task/task/v3/cmd/task@latest
go get go.yaml.in/yaml/v3 github.com/yuin/goldmark

go tool task --version              # tools run via `go tool`, nothing installed globally
```

### `cmd/tracker` flags

```
--addr     127.0.0.1:8765        listen address
--data     <dir>                 data directory (default: data/ next to the executable)
--open                           open the default browser once /healthz responds
--dev                            serve static files from disk, no caching, debug logging
--demo                           copy the embedded sample data (internal/demo) to a temp dir and use it
--version                        print version (set via -ldflags -X main.version=…)
```

- Graceful shutdown uses `signal.NotifyContext` and `srv.Shutdown`.
- If the port is busy, fail with a clear message: "already running? open http://127.0.0.1:8765".
- Opening the browser on Windows: `rundll32 url.dll,FileProtocolHandler <url>`. On macOS use `open`, on Linux `xdg-open`.

### `Taskfile.yml` (sketch)

```yaml
version: '3'
vars:
  VERSION: { sh: git describe --tags --always --dirty }
tasks:
  generate: go tool templ generate
  dev:      go tool air
  run:
    deps: [generate]
    cmds: [go run ./cmd/tracker --data ./data --open]
  test:
    deps: [generate]
    cmds: [go test ./...]
  lint:     go vet ./...
  build:
    deps: [generate]
    cmds:
      - go build -trimpath -ldflags "-s -w -X main.version={{.VERSION}}" -o dist/tracker{{exeExt}} ./cmd/tracker
  release:
    deps: [generate]
    cmds:
      - for:
          matrix:
            OS:   [windows, darwin, linux]
            ARCH: [amd64, arm64]
        task: build-target
        vars: { OS: '{{.ITEM.OS}}', ARCH: '{{.ITEM.ARCH}}' }
  build-target:
    internal: true
    env: { GOOS: '{{.OS}}', GOARCH: '{{.ARCH}}', CGO_ENABLED: '0' }   # per-command env, never `go env -w`
    cmds:
      - go build -trimpath -ldflags "-s -w -X main.version={{.VERSION}}" -o dist/tracker-{{.OS}}-{{.ARCH}}{{if eq .OS "windows"}}.exe{{end}} ./cmd/tracker
```

### `.air.toml` (sketch)

```toml
root = "."
tmp_dir = "tmp"

[build]
  pre_cmd      = ["go tool templ generate"]
  cmd          = "go build -o ./tmp/tracker.exe ./cmd/tracker"
  bin          = "./tmp/tracker.exe"
  args_bin     = ["--data", "./data", "--dev"]
  include_ext  = ["go", "templ", "css", "js"]
  exclude_dir  = ["tmp", "dist", "data", "testdata"]
  exclude_regex = ["_templ\\.go$", "_test\\.go$"]   # prevents a generate → rebuild loop

[proxy]                 # browser auto-reload: open http://127.0.0.1:8766 during dev
  enabled    = true
  proxy_port = 8766
  app_port   = 8765
```

### `.gitignore`

```
/data/
/dist/
/tmp/
*.exe
.idea/workspace.xml
.idea/shelf/
```
**Never commit `data/`.** It holds personal information, and the repo has a GPL licence and may become public. Keep your data in its own private git repo if you want history.

### Distribution
- `go tool task build` → `dist/tracker.exe` (~10–15 MB). Copy it anywhere and double-click it; it creates `data/` next to itself.
- Keep the console window in v1: it shows logs and Ctrl+C stops the app. A system-tray build (`-H=windowsgui` plus a tray library) is a later option.

---

## 10. GoLand setup

1. **Project files**: the current `.idea/` was created by IntelliJ as a Java module (JDK 21) and isn't tracked by git. Close the project in IntelliJ, delete `.idea/`, then open the folder in GoLand so it creates a clean Go project.
2. **SDK**: Settings → Go → GOROOT → pick or download **Go 1.27.x**. Go modules integration is on by default.
3. **templ plugin**: install **templ** from the Marketplace ([templ-go/templ-jetbrains](https://github.com/templ-go/templ-jetbrains)) for syntax highlighting and LSP. The plugin needs a `templ` executable. If it can't find one, run `go install github.com/a-h/templ/cmd/templ@<version from go.mod>` so it's on PATH, and keep that version in sync with `go.mod`.
4. **Run configurations**
    - *tracker (debug)*: Go Build, package `./cmd/tracker`, program arguments `--data ./data --dev --open`, working directory = project root. Under *Before launch*, add an External Tool that runs `go tool templ generate`. Use this one for breakpoints.
    - *dev (live reload)*: Shell Script → `go tool task dev`. Use this one for UI work.
    - *tests*: Go Test, directory `./...`.
5. **Optional File Watcher** for `*.templ`: program `go`, arguments `tool templ generate -f $FilePath$`, so generated code refreshes as you type even outside air.
6. Mark `data/`, `dist/` and `tmp/` as **Excluded** (right-click → Mark Directory as → Excluded).

---

## 11. Testing strategy

| Layer | What to test | How |
|---|---|---|
| model | Date/DateTime YAML and form round-trips, validation rules, `Events()`, overdue/upcoming/stale logic | table-driven tests with an injected clock |
| store | save → load round-trip (golden YAML in `testdata/`), atomic write leaves no partial files, slug collisions, malformed file is reported and never overwritten, unknown enum values preserved, path traversal rejected (`../x`), trash on delete | `t.TempDir()` as data dir |
| web | create → detail → edit each section → status change → upload/download, 422 re-render with errors, cross-origin POST rejected, bad Host rejected | `httptest.NewServer` + `net/http` client |
| views | key components render expected markup (pills, rating, empty states) | render to `bytes.Buffer`, assert substrings |
| manual | full click-through with `--demo` data in a browser | each milestone's acceptance list below |

---

## 12. Milestones

Each milestone ends with a working, runnable app. Tick the boxes as you go.

### M0: Skeleton (½ day)
- [ ] Install Go 1.27, set up GoLand (§10), `go mod init`, add tool dependencies (§9)
- [ ] `cmd/tracker/main.go` with flags, slog, graceful shutdown, `--open`, `/healthz`
- [ ] `web/embed.go` + vendored htmx 2.x, Pico CSS, SortableJS (+ `VERSIONS.md`)
- [ ] `views/layout.templ` with nav + an empty dashboard page
- [ ] Security middleware: CrossOriginProtection + Host allowlist
- [ ] `Taskfile.yml`, `.air.toml`, `.gitignore`, short `README.md`
- **Done when:** `go tool task dev` live-reloads the page, and `go tool task build` produces a single `tracker.exe` that runs from any folder.

### M1: Data layer (1–2 days)
- [ ] `model`: all structs (§5), Date/DateTime, SalaryRange, lookups with defaults, validation
- [ ] `store`: open/create data dir, write default `config.yaml`, load all companies and applications into memory, record load errors
- [ ] CRUD for applications and companies, slug IDs, atomic writes with Windows retry, trash
- [ ] `internal/demo/sample/` with about 8 realistic (fake) applications across all statuses, embedded with `//go:embed`
- [ ] Unit tests for model and store (§11)
- **Done when:** `go tool task test` is green, and a test-created application produces readable YAML that matches the example in §4.

### M2: Core UI (2–3 days)
- [ ] Quick add (company autocomplete via `<datalist>`, or create a new company inline)
- [ ] Applications table: sort, filter, search, closed toggle, `hx-push-url`
- [ ] Detail page with all six section cards: view partial, edit partial, save with 422 error re-render
- [ ] Inline priority and status selects, "Contacted today" button
- [ ] Companies list, page, create and edit
- [ ] Markdown rendering for long-text fields
- **Done when:** every field in §5 can be entered, saved, viewed and edited; data survives a restart; hand-editing a YAML file and clicking Reload shows the change.

### M3: Repeatables and files (1–2 days)
- [ ] Repeatable rows: recruiters, interviewers, interview rounds (labelled Interview 1/2/3…), Q&A (with optional round), extra materials
- [ ] Referrer and hiring manager single-contact editors
- [ ] Attachment upload, download and delete via `os.Root`; posting copy as an uploaded file or pasted text (`posting.md`)
- [ ] CV library (`data/cv/`) and CV version picker; "used in" reverse lookup
- [ ] Sandboxed serving for HTML posting copies
- **Done when:** an application can hold 3+ interview rounds with Q&A, multiple recruiters, a posting PDF and a cover letter, all saved under its own folder.

### M4: Dashboard, board, calendar (1–2 days)
- [ ] `model.Events()` and dashboard queries (overdue, upcoming, stale, pipeline counts and conversion)
- [ ] Dashboard page
- [ ] Kanban board with SortableJS drag → status POST, plus a no-JS fallback select
- [ ] Calendar agenda view + `/calendar.ics` export
- [ ] Optional: auto-set `dateApplied` when status first moves to `applied`; offer to set the matching status when Decision changes
- **Done when:** the dashboard correctly shows overdue and upcoming items for the demo data (verified by tests with a fixed clock), and dragging a card updates the YAML file.

### M5: Polish and packaging (1–2 days)
- [ ] CSV export, ZIP backup download
- [ ] Concurrent and external edit detection (`rev` + mtime check), "Reload from disk"
- [ ] Load-error banner, empty states, toast notifications, keyboard shortcuts (`n` new, `/` search)
- [ ] Strict CSP for the app's own pages
- [ ] `--demo` mode, `--version`, cross-compiled `release` task
- [ ] README: install, run, data format, backup advice
- **Done when:** a fresh machine can download `tracker.exe`, double-click it, and track an application end to end with no other installs.

### Later ideas (not scheduled)
- Shared contacts directory (`data/contacts/`) referenced from applications, so one recruiter can be linked across several roles
- "Fetch & save posting" from a URL (best effort; LinkedIn and similar sites need login, so fall back to paste)
- Auto-commit `data/` to git on every save
- Windows toast reminders for due follow-ups, system tray icon
- In-app editor for `config.yaml` lists
- Stats page: time-to-response, channel effectiveness, rejection reasons
- Import from CSV or spreadsheet
- htmx 4 migration once it becomes the `latest` release (expected early 2027)

---

## 13. Decisions made (change if you disagree)

1. **Interview 1/2/3 dates → a list of rounds.** The UI still shows "Interview 1, 2, 3", but a fourth round doesn't need a schema change. Each round can hold its type, interviewers and notes, and Q&A can point at a round.
2. **Questions asked / answers → a list of `{round, question, answer}`** rather than one text blob. This makes it easy to review before the next interview.
3. **Salaries → structured `SalaryRange`** (min/max/currency/period + note). Ranges stay sortable and comparable, and the note keeps raw text like "DOE".
4. **Notice period / start date → two fields** (free-text notice period + real start date) so the start date can appear in the calendar.
5. **Company is its own file**, shared by all applications to that company. Contacts stay **inline** per application in v1, for simplicity and self-contained files.
6. **Status vs Decision**: both are kept as specified. Status is the pipeline stage; Decision is the final outcome. Changing Decision *offers* to set the matching status (one click) but never changes it automatically.
7. **YAML over JSON or Markdown**: easiest to hand-edit with multi-line notes, and it decodes straight into typed structs.
8. **Generated templ code is committed**, so `go build` / `go install` work without the templ CLI.
9. **htmx 2.x** now; htmx 4 later (see Later ideas).

## 14. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Windows file locks (antivirus, indexer, OneDrive) break renames | Retry with backoff; recommend keeping `data/` outside OneDrive-synced folders, or accept OneDrive and rely on retries |
| Hand-edited YAML breaks parsing | Skip the file, show it in a banner, never overwrite it; error message includes the line number |
| Two app instances on the same data dir | Port binding blocks a second instance on the same port; M5: `data/.lock` with PID warning |
| Schema changes later | `schemaVersion` in every file, plus a migration step in `store.Load` |
| Personal data leaks into the public repo | `/data/` in `.gitignore` from M0; `--demo` sample data and test fixtures are fake only |
