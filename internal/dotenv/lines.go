package dotenv

import "strings"

// Lines splits content into lines without their newline characters.
// trailing reports whether content ended with a newline. The split is
// preserved by Join so version bytes stay stable.
func Lines(content string) (lines []string, trailing bool) {
	if content == "" {
		return nil, false
	}
	trailing = strings.HasSuffix(content, "\n")
	parts := strings.Split(content, "\n")
	if trailing {
		parts = parts[:len(parts)-1]
	}
	return parts, trailing
}

// Join is the inverse of Lines.
func Join(lines []string, trailing bool) string {
	s := strings.Join(lines, "\n")
	if trailing {
		s += "\n"
	}
	return s
}
