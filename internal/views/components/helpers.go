// Package components holds small reusable view pieces: pills, ratings,
// form inputs, contact rows and Markdown.
package components

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

// md renders Markdown. Raw HTML and dangerous URLs (javascript: etc.) are
// dropped, which is goldmark's default.
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

// Markdown renders s as HTML inside <div class="md">.
func Markdown(s string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		if _, err := io.WriteString(w, `<div class="md">`); err != nil {
			return err
		}
		if err := md.Convert([]byte(s), w); err != nil {
			return err
		}
		_, err := io.WriteString(w, `</div>`)
		return err
	})
}

// Option is one <option> in a select.
type Option struct {
	Value, Label string
}

// Strings turns a config list into options.
func Strings(list []string) []Option {
	out := make([]Option, len(list))
	for i, s := range list {
		out[i] = Option{s, s}
	}
	return out
}

// Statuses returns the configured statuses as options.
func Statuses(cfg *model.Config) []Option {
	out := make([]Option, len(cfg.Statuses))
	for i, s := range cfg.Statuses {
		out[i] = Option{s.ID, s.Label}
	}
	return out
}

// Priorities returns the configured priorities as options.
func Priorities(cfg *model.Config) []Option {
	out := make([]Option, len(cfg.Priorities))
	for i, p := range cfg.Priorities {
		out[i] = Option{p.ID, p.Label}
	}
	return out
}

// withCurrent keeps a value that is no longer in config.yaml selectable, so
// editing a form never drops it.
func withCurrent(opts []Option, value string) []Option {
	if value == "" {
		return opts
	}
	for _, o := range opts {
		if o.Value == value {
			return opts
		}
	}
	return append(append([]Option(nil), opts...), Option{value, value + " (not in config)"})
}

// Invalid marks an input as invalid for Pico's error styling.
func Invalid(err string) templ.Attributes {
	if err == "" {
		return nil
	}
	return templ.Attributes{"aria-invalid": "true"}
}

// Stars renders a 1–5 rating as filled and empty stars: ★★★ and ☆☆.
func Stars(n int) (filled, empty string) {
	n = max(0, min(5, n))
	return strings.Repeat("★", n), strings.Repeat("☆", 5-n)
}

func ratingLabel(i int) string {
	if i == 0 {
		return "–"
	}
	return strconv.Itoa(i)
}

// IntValue formats n for a number input, leaving it blank when zero.
func IntValue(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// FileSize formats a byte count: "820 B", "12 KB", "3.4 MB".
func FileSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%d KB", (n+512)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// Relative describes d relative to today: "today", "in 3 days", "2 days ago".
func Relative(d, today model.Date) string {
	if d.IsZero() {
		return ""
	}
	switch n := today.DaysUntil(d); {
	case n == 0:
		return "today"
	case n == 1:
		return "tomorrow"
	case n == -1:
		return "yesterday"
	case n > 0:
		return fmt.Sprintf("in %d days", n)
	default:
		return fmt.Sprintf("%d days ago", -n)
	}
}

// DayHeading formats a day for agenda lists: "Wed 14 Oct".
func DayHeading(d, today model.Date) string {
	if d.Year() != today.Year() {
		return d.Format("Mon 2 Jan 2006")
	}
	return d.Format("Mon 2 Jan")
}

// ShortDate formats a date for tables: "14 Oct", with the year if it isn't
// the current one.
func ShortDate(d, today model.Date) string {
	if d.IsZero() {
		return ""
	}
	if d.Year() != today.Year() {
		return d.Format("2 Jan 2006")
	}
	return d.Format("2 Jan")
}
