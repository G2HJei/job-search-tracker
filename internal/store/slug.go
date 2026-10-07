package store

import (
	"strconv"
	"strings"
	"unicode"
)

var translit = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ą': "a", 'ă': "a",
	'æ': "ae", 'ç': "c", 'ć': "c", 'č': "c", 'ď': "d", 'đ': "d", 'ð': "d",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ę': "e", 'ě': "e", 'ğ': "g",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ı': "i", 'ł': "l", 'ñ': "n", 'ń': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ő': "o", 'œ': "oe",
	'ř': "r", 'ś': "s", 'š': "s", 'ş': "s", 'ș': "s", 'ß': "ss", 'ť': "t", 'ț': "t", 'þ': "th",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ů': "u", 'ű': "u", 'ý': "y", 'ÿ': "y",
	'ź': "z", 'ż': "z", 'ž': "z",
	// Cyrillic (Bulgarian/Russian)
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "zh", 'з': "z",
	'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r",
	'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sht",
	'ъ': "a", 'ы': "y", 'ь': "y", 'э': "e", 'ю': "yu", 'я': "ya",
	'&': "and", '+': "plus",
}

// Slugify turns s into lowercase ASCII words joined by hyphens, at most
// maxLen bytes long (cut at a word boundary where possible).
func Slugify(s string, maxLen int) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		var part string
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			part = string(r)
		case translit[r] != "":
			part = translit[r]
		}
		if part == "" {
			dash = b.Len() > 0
			continue
		}
		if dash {
			b.WriteByte('-')
			dash = false
		}
		b.WriteString(part)
	}
	out := b.String()
	if len(out) > maxLen {
		out = out[:maxLen]
		if i := strings.LastIndexByte(out, '-'); i > maxLen/2 {
			out = out[:i]
		}
		out = strings.TrimRight(out, "-")
	}
	return out
}

// uniqueID returns base, or base-2, base-3, … if taken.
func uniqueID(base string, taken func(string) bool) string {
	id := base
	for n := 2; taken(id); n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}
