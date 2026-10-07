package model

import (
	"strconv"
	"strings"
)

type SalaryRange struct {
	Min      int    `yaml:"min,omitempty"`
	Max      int    `yaml:"max,omitempty"`
	Currency string `yaml:"currency,omitempty"`
	Period   string `yaml:"period,omitempty"` // year | month | day | hour
	Note     string `yaml:"note,omitempty"`   // "DOE", "+10% bonus", raw text from the ad
}

// IsZero reports whether no amount and no note is set. A currency or period
// on its own (left over from form defaults) counts as empty.
func (s SalaryRange) IsZero() bool {
	return s.Min == 0 && s.Max == 0 && strings.TrimSpace(s.Note) == ""
}

var currencySymbols = map[string]string{"GBP": "£", "USD": "$", "EUR": "€"}

// String formats the range for display, e.g. "£80,000–95,000 / year (DOE)".
func (s SalaryRange) String() string {
	if s.IsZero() {
		return ""
	}
	var b strings.Builder
	if s.Min != 0 || s.Max != 0 {
		sym, ok := currencySymbols[strings.ToUpper(s.Currency)]
		if ok {
			b.WriteString(sym)
		}
		switch {
		case s.Min != 0 && s.Max != 0 && s.Min != s.Max:
			b.WriteString(groupThousands(s.Min) + "–" + groupThousands(s.Max))
		case s.Min != 0:
			b.WriteString(groupThousands(s.Min))
			if s.Max == 0 {
				b.WriteString("+")
			}
		default:
			b.WriteString("up to " + groupThousands(s.Max))
		}
		if !ok && s.Currency != "" {
			b.WriteString(" " + s.Currency)
		}
		if s.Period != "" {
			b.WriteString(" / " + s.Period)
		}
	}
	if note := strings.TrimSpace(s.Note); note != "" {
		if b.Len() > 0 {
			b.WriteString(" (" + note + ")")
		} else {
			b.WriteString(note)
		}
	}
	return b.String()
}

func groupThousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var out []byte
	for i := range len(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
