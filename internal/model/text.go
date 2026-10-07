package model

import "strings"

// NormalizeText tidies multi-line input from a textarea so it is stored as a
// readable YAML block: CRLF becomes LF, trailing spaces are removed from each
// line (they force the encoder into a quoted style), and leading and trailing
// blank lines are dropped. Multi-line text keeps one final newline.
func NormalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	switch len(lines) {
	case 0:
		return ""
	case 1:
		return lines[0]
	}
	return strings.Join(lines, "\n") + "\n"
}

// NormalizeLine tidies single-line input.
func NormalizeLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
