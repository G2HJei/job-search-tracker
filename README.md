# Job Search Tracker

A local web app for tracking job applications. One executable serves a web UI on
`127.0.0.1`, and your data lives next to it as human-readable YAML files. There is no database,
no account and no cloud.

Built with Go, [templ](https://templ.guide) and [htmx](https://htmx.org). All frontend assets are
embedded in the binary, so it works offline.

## Features

- **Dashboard**: overdue actions, upcoming events, applications that need a follow-up, and pipeline
  counts with stage conversion.
- **Applications table**: sort, filter, free-text search (company, title, notes, contacts) and a
  "show closed" toggle. Filtered URLs can be bookmarked.
- **Application page** with six section cards (Company, Job Application, Contacts, Hiring Process,
  Compensation, Other) that you edit in place. It also has inline status/priority selects and a
  "Contacted today" button.
- **Repeatable entries**: recruiters, interviewers, interview rounds (Interview 1, 2, 3, …),
  questions and answers per round, and extra materials sent.
- **Attachments**: upload the posting (PDF/HTML/MHTML) or paste its text, cover letters and
  take-home work. Each application has its own folder.
- **CV library** with a "used in" view.
- **Kanban board** with drag-and-drop between statuses.
- **Calendar** agenda plus an `.ics` export for Outlook or Google Calendar.
- **Export**: CSV of every application, and a ZIP backup of the whole data folder.

## Install and run

Download `tracker.exe` (or build it, see below), put it in any folder and double-click it. It
creates a `data/` folder next to itself and prints the address to open:

```
  Job Search Tracker is running at http://127.0.0.1:8765
  Data: C:\Tools\tracker\data
  Press Ctrl+C to stop.
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `--addr` | `127.0.0.1:8765` | Listen address. Other interfaces only if you ask for them. |
| `--data` | `$JST_DATA_DIR`, else `data/` next to the executable | Data directory |
| `--open` | on | Open the browser once the server is ready; `--open=false` to skip |
| `--demo` | off | Run on a temporary copy of built-in fake data; changes are discarded on exit |
| `--dev` | off | Serve static files from `web/static` on disk, no caching, debug logs |
| `--version` | | Print the version |

Keyboard shortcuts: `n` starts a new application, and `/` focuses the search box.

## Your data

```
data/
├── config.yaml                 # dropdown lists and settings (written with defaults on first run)
├── companies/acme.yaml         # one file per company, shared by its applications
├── applications/
│   └── 2026-10-07-acme-backend-engineer/
│       ├── application.yaml    # everything except the company details
│       └── files/              # posting copy, cover letter, …
├── cv/                         # your CV library
└── .trash/                     # deleted applications and files end up here, never hard-deleted
```

- **Plain YAML.** You can read, edit and diff it. Long text uses `|` blocks, and dates look like
  `2026-10-14` (or `2026-10-14T14:00` when there is a time).
- **IDs never change.** An application's folder is `YYYY-MM-DD-company-title` from the day it was
  created. Renaming the job title does not rename the folder.
- **Edit by hand, then reload.** After editing files in an editor, click *Reload from disk* (in
  Settings or the banner). If a file doesn't parse, or has keys the app doesn't know, it is skipped
  and listed in a banner with the line number. The app never overwrites it.
- **Unknown values are kept.** If a status, channel or other value is no longer in `config.yaml`, it
  is shown with a wavy underline and kept as-is.
- **Conflicts are detected.** If the same section changed in another tab, or the file changed on
  disk, the save stops and explains what happened instead of overwriting.
- **Lookup lists** (statuses, priorities, channels, contract types, decisions, company sizes,
  interview types, salary periods) are edited in `config.yaml`. The order of `statuses` is the board
  column order, and `closed: true` hides a status from the active views.

### Backups

- Use **Settings → Download backup (.zip)** now and then. Or better, keep history by turning the
  data folder into its own private git repository:

  ```powershell
  cd data
  git init
  git add -A; git commit -m "snapshot"
  ```

- **Never commit `data/` to this app's repository.** It holds personal information, and `/data/` is
  in `.gitignore`.
- Keep `data/` outside OneDrive-synced folders if you can. Sync tools can briefly lock files; the app
  retries writes, but it's best avoided.

## Security

The app is meant for your machine only:

- It listens on `127.0.0.1` by default.
- Cross-origin form posts are rejected (`http.CrossOriginProtection`), and requests with a
  foreign `Host` header are rejected too, which blocks DNS rebinding.
- A strict Content-Security-Policy applies (`default-src 'self'`, no inline scripts).
- Saved HTML job postings open in a sandbox, so their scripts can't run. Other files are
  downloaded rather than displayed.
- File access is confined to the data folder (`os.Root`). Uploads are limited to 25 MB.

## Development

Requires Go 1.27+. The dev tools (`templ`, `air`, `task`) are tool dependencies in `go.mod`, so run
them with `go tool …` and nothing needs installing globally. On this machine, `air` on PATH is a
JetBrains Toolbox script, so always use `go tool air`.

```powershell
go tool task dev       # live reload; open http://127.0.0.1:8766 for browser auto-refresh
go tool task demo      # run on the sample data
go tool task test      # templ generate + go test ./...
go tool task lint      # go vet
go tool task build     # dist/tracker.exe with the version from git describe
go tool task release   # cross-compile for Windows, macOS and Linux (amd64 + arm64)
```

Generated `*_templ.go` files are committed, so plain `go build ./cmd/tracker` works without templ.
After changing a `.templ` file, run `go tool task generate` (air and the build tasks do this for
you).

Layout:

| Path | What |
|---|---|
| `cmd/tracker` | flags, wiring, graceful shutdown, `--open` |
| `internal/model` | domain types, dates, validation, events, dashboard logic (no I/O) |
| `internal/store` | the only code that touches `data/`: load, atomic writes, trash, files |
| `internal/web` | routes, handlers, form binding, htmx-aware rendering, security middleware |
| `internal/views` | templ pages; `components/` (inputs, pills, Markdown) and `sections/` (the six cards) |
| `internal/demo` | embedded fake sample data for `--demo` and tests |
| `web/static` | `app.css`, `app.js` and the vendored htmx, SortableJS and Pico CSS (see `vendor/VERSIONS.md`) |

GoLand: the shared run configurations in `.run/` give you *tracker (debug)*, *dev (live reload)* and
*tests*. Install the **templ** plugin from the Marketplace for `.templ` syntax highlighting. If
the plugin can't find `templ`, install the version from `go.mod` with
`go install github.com/a-h/templ/cmd/templ@v0.3.1070`.

## License

GPL-3.0. See [LICENSE](LICENSE). The vendored libraries are 0BSD (htmx) and MIT (SortableJS, Pico CSS).
