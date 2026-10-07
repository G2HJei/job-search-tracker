package model

import (
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestDateYAMLRoundTrip(t *testing.T) {
	type doc struct {
		D  Date     `yaml:"d,omitempty"`
		DT DateTime `yaml:"dt,omitempty"`
		DO DateTime `yaml:"do,omitempty"`
		Z  Date     `yaml:"z,omitempty"`
	}
	in := doc{
		D:  NewDate(2026, 10, 14),
		DT: NewDateTime(2026, 10, 14, 14, 0),
		DO: DateOnly(NewDate(2026, 10, 16)),
	}
	out, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "d: 2026-10-14\ndt: 2026-10-14T14:00\ndo: 2026-10-16\n"
	if string(out) != want {
		t.Fatalf("marshal:\n got %q\nwant %q", out, want)
	}
	var back doc
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back != in {
		t.Fatalf("round trip: got %+v want %+v", back, in)
	}
}

func TestDateYAMLAcceptsQuotedAndVariants(t *testing.T) {
	var v struct {
		A Date     `yaml:"a"`
		B DateTime `yaml:"b"`
		C DateTime `yaml:"c"`
		D DateTime `yaml:"d"`
	}
	src := `a: "2026-10-14"
b: '2026-10-14T09:30'
c: 2026-10-14T09:30:00+01:00
d: 2026-10-14 09:30
`
	if err := yaml.Unmarshal([]byte(src), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != NewDate(2026, 10, 14) {
		t.Errorf("a = %v", v.A)
	}
	want := NewDateTime(2026, 10, 14, 9, 30)
	for name, got := range map[string]DateTime{"b": v.B, "c": v.C, "d": v.D} {
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestDateYAMLErrorHasLine(t *testing.T) {
	var v struct {
		A Date `yaml:"a"`
	}
	err := yaml.Unmarshal([]byte("x: 1\na: 2026-13-45\n"), &v)
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("want error mentioning line 2, got %v", err)
	}
}

func TestDateForm(t *testing.T) {
	var d Date
	if err := d.ParseForm("2026-10-07"); err != nil || d != NewDate(2026, 10, 7) {
		t.Fatalf("ParseForm: %v %v", d, err)
	}
	if d.FormValue() != "2026-10-07" {
		t.Fatalf("FormValue = %q", d.FormValue())
	}
	if err := d.ParseForm(""); err != nil || !d.IsZero() {
		t.Fatalf("empty: %v %v", d, err)
	}
	if err := d.ParseForm("07/10/2026"); err == nil {
		t.Fatal("want error for bad date")
	}

	var dt DateTime
	if err := dt.ParseForm("2026-10-14T14:00"); err != nil || dt != NewDateTime(2026, 10, 14, 14, 0) {
		t.Fatalf("ParseForm: %v %v", dt, err)
	}
	if dt.FormValue() != "2026-10-14T14:00" || dt.Clock() != "14:00" {
		t.Fatalf("FormValue = %q", dt.FormValue())
	}
	// Date-only values are shown as midnight and read back as date-only.
	dateOnly := DateOnly(NewDate(2026, 10, 16))
	if err := dt.ParseForm(dateOnly.FormValue()); err != nil || dt != dateOnly || dt.HasTime() {
		t.Fatalf("date-only round trip: %v %v", dt, err)
	}
}

func TestDateArithmetic(t *testing.T) {
	a := NewDate(2026, 10, 7)
	b := NewDate(2026, 11, 3) // crosses the end of BST
	if n := a.DaysUntil(b); n != 27 {
		t.Fatalf("DaysUntil = %d", n)
	}
	if a.AddDays(27) != b {
		t.Fatalf("AddDays = %v", a.AddDays(27))
	}
	if !a.Before(b) || a.Compare(b) != -1 || (Date{}).Compare(a) != -1 {
		t.Fatal("ordering")
	}
	if got := DateOf(time.Date(2026, 10, 7, 23, 59, 0, 0, time.FixedZone("x", 3600))); got != a {
		t.Fatalf("DateOf = %v", got)
	}
	if NewDateTime(2026, 10, 7, 9, 0).Date() != a {
		t.Fatal("DateTime.Date")
	}
}
