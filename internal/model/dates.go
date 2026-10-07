package model

import (
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	dateLayout     = "2006-01-02"
	dateTimeLayout = "2006-01-02T15:04"
)

// Date is a calendar date without a time zone, stored as "2006-01-02".
// The zero value means "not set".
type Date struct {
	t time.Time // midnight UTC; zero = unset
}

// NewDate returns the given calendar date.
func NewDate(year int, month time.Month, day int) Date {
	return Date{t: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// DateOf returns the calendar date of t in t's own location.
func DateOf(t time.Time) Date {
	if t.IsZero() {
		return Date{}
	}
	y, m, d := t.Date()
	return NewDate(y, m, d)
}

// Today returns the current date in time.Local according to now.
func Today(now func() time.Time) Date {
	return DateOf(now().In(time.Local))
}

// ParseDate parses "2006-01-02". It also accepts a datetime or RFC 3339
// value and keeps only the date part. An empty string gives the zero Date.
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, nil
	}
	if t, err := time.Parse(dateLayout, s); err == nil {
		return DateOf(t), nil
	}
	if dt, err := ParseDateTime(s); err == nil {
		return dt.Date(), nil
	}
	return Date{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s)
}

func (d Date) IsZero() bool { return d.t.IsZero() }

// String returns "2006-01-02", or "" when unset.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(dateLayout)
}

// FormValue returns the value for an <input type="date">.
func (d Date) FormValue() string { return d.String() }

// ParseForm sets d from an <input type="date"> value.
func (d *Date) ParseForm(s string) error {
	v, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}

func (d Date) Year() int             { return d.t.Year() }
func (d Date) Month() time.Month     { return d.t.Month() }
func (d Date) Day() int              { return d.t.Day() }
func (d Date) Weekday() time.Weekday { return d.t.Weekday() }
func (d Date) Before(o Date) bool    { return d.t.Before(o.t) }
func (d Date) After(o Date) bool     { return d.t.After(o.t) }
func (d Date) Equal(o Date) bool     { return d.t.Equal(o.t) }
func (d Date) AddDays(n int) Date    { return Date{t: d.t.AddDate(0, 0, n)} }
func (d Date) Format(layout string) string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(layout)
}

// Compare returns -1, 0 or +1. Unset dates sort before set ones.
func (d Date) Compare(o Date) int { return d.t.Compare(o.t) }

// DaysUntil returns the number of days from d to o (negative if o is earlier).
func (d Date) DaysUntil(o Date) int {
	return int(o.t.Sub(d.t).Hours() / 24)
}

// Time returns midnight of d in loc.
func (d Date) Time(loc *time.Location) time.Time {
	return time.Date(d.t.Year(), d.t.Month(), d.t.Day(), 0, 0, 0, 0, loc)
}

// Display formats the date for humans: "14 Oct 2026".
func (d Date) Display() string { return d.Format("2 Jan 2006") }

// MarshalYAML emits an unquoted scalar. Returning a plain string would make
// the encoder quote it, because "2026-10-14" looks like a timestamp.
func (d Date) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: d.String()}, nil
}

func (d *Date) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: date must be a scalar", node.Line)
	}
	v, err := ParseDate(node.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", node.Line, err)
	}
	*d = v
	return nil
}

// DateTime is a local wall-clock time without a zone, stored as
// "2006-01-02T15:04" (the <input type="datetime-local"> format). It may also
// hold a date only.
type DateTime struct {
	t       time.Time // wall clock as UTC; zero = unset
	hasTime bool
}

// NewDateTime returns a DateTime with a time of day.
func NewDateTime(year int, month time.Month, day, hour, min int) DateTime {
	return DateTime{t: time.Date(year, month, day, hour, min, 0, 0, time.UTC), hasTime: true}
}

// DateOnly returns a DateTime that holds only a date.
func DateOnly(d Date) DateTime {
	if d.IsZero() {
		return DateTime{}
	}
	return DateTime{t: d.t}
}

var dateTimeLayouts = []string{
	dateTimeLayout,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02 15:04:05",
}

// ParseDateTime parses "2006-01-02T15:04", a few close variants, RFC 3339
// (the zone is dropped and the wall clock kept) or a date only.
func ParseDateTime(s string) (DateTime, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DateTime{}, nil
	}
	if t, err := time.Parse(dateLayout, s); err == nil {
		return DateOnly(DateOf(t)), nil
	}
	for _, layout := range dateTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return NewDateTime(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()), nil
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return NewDateTime(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()), nil
	}
	return DateTime{}, fmt.Errorf("invalid date/time %q (want YYYY-MM-DDTHH:MM)", s)
}

func (dt DateTime) IsZero() bool  { return dt.t.IsZero() }
func (dt DateTime) HasTime() bool { return dt.hasTime }

// Date returns the date part.
func (dt DateTime) Date() Date {
	if dt.IsZero() {
		return Date{}
	}
	return DateOf(dt.t)
}

// String returns "2006-01-02T15:04", "2006-01-02" for date-only values, or "".
func (dt DateTime) String() string {
	switch {
	case dt.IsZero():
		return ""
	case dt.hasTime:
		return dt.t.Format(dateTimeLayout)
	default:
		return dt.t.Format(dateLayout)
	}
}

// FormValue returns the value for an <input type="datetime-local">, which
// needs a time; date-only values are shown as midnight.
func (dt DateTime) FormValue() string {
	if dt.IsZero() {
		return ""
	}
	return dt.t.Format(dateTimeLayout)
}

// ParseForm sets dt from an <input type="datetime-local"> value. Midnight is
// read back as "date only", which is how FormValue shows date-only values.
func (dt *DateTime) ParseForm(s string) error {
	v, err := ParseDateTime(s)
	if err != nil {
		return err
	}
	if v.hasTime && v.t.Hour() == 0 && v.t.Minute() == 0 {
		v.hasTime = false
	}
	*dt = v
	return nil
}

// Clock returns "15:04", or "" for date-only values.
func (dt DateTime) Clock() string {
	if !dt.hasTime {
		return ""
	}
	return dt.t.Format("15:04")
}

// Display formats for humans: "14 Oct 2026 14:00" or "14 Oct 2026".
func (dt DateTime) Display() string {
	if dt.IsZero() {
		return ""
	}
	if dt.hasTime {
		return dt.t.Format("2 Jan 2006 15:04")
	}
	return dt.t.Format("2 Jan 2006")
}

// Compare orders by wall clock; a date-only value sorts at the start of its day.
func (dt DateTime) Compare(o DateTime) int { return dt.t.Compare(o.t) }

// Time returns the wall-clock time in loc.
func (dt DateTime) Time(loc *time.Location) time.Time {
	return time.Date(dt.t.Year(), dt.t.Month(), dt.t.Day(), dt.t.Hour(), dt.t.Minute(), 0, 0, loc)
}

// AddDays shifts the value by n days, keeping the time of day.
func (dt DateTime) AddDays(n int) DateTime {
	if dt.IsZero() {
		return dt
	}
	return DateTime{t: dt.t.AddDate(0, 0, n), hasTime: dt.hasTime}
}

func (dt DateTime) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: dt.String()}, nil
}

func (dt *DateTime) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: date/time must be a scalar", node.Line)
	}
	v, err := ParseDateTime(node.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", node.Line, err)
	}
	*dt = v
	return nil
}
