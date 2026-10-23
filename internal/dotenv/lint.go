package dotenv

import (
	"fmt"
	"regexp"
	"strings"
)

// Severity is a lint finding level.
type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Issue is one syntax finding. Line is 1-based.
type Issue struct {
	Line     int
	Severity Severity
	Message  string
}

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Lint checks dotenv syntax. Comments, blank lines, optional spaces around
// "=", and an optional export prefix are accepted. Values are not rewritten.
func Lint(content string) []Issue {
	lines, _ := Lines(content)
	var issues []Issue
	seen := map[string]int{}
	inQuote := byte(0)
	startLine := 0
	var key string

	flush := func(line int) {
		if key == "" {
			return
		}
		if prev, ok := seen[key]; ok {
			issues = append(issues, Issue{
				Line:     line,
				Severity: Warning,
				Message:  fmt.Sprintf("duplicate key %s (also on line %d)", key, prev),
			})
		}
		seen[key] = line
		key = ""
	}

	for i, line := range lines {
		lineNo := i + 1
		if inQuote != 0 {
			if quoteClosed(line, inQuote) {
				inQuote = 0
				flush(startLine)
			}
			continue
		}
		trim := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		body := trim
		if strings.HasPrefix(body, "export ") {
			body = strings.TrimSpace(strings.TrimPrefix(body, "export "))
		} else if body == "export" {
			issues = append(issues, Issue{Line: lineNo, Severity: Error, Message: "export without an assignment"})
			continue
		}
		eq := strings.IndexByte(body, '=')
		if eq < 0 {
			issues = append(issues, Issue{Line: lineNo, Severity: Error, Message: "missing '='"})
			continue
		}
		key = strings.TrimSpace(body[:eq])
		rawVal := body[eq+1:]
		val := strings.TrimSpace(rawVal)
		if key == "" {
			issues = append(issues, Issue{Line: lineNo, Severity: Error, Message: "empty key"})
			key = ""
			continue
		}
		if !keyPattern.MatchString(key) {
			issues = append(issues, Issue{Line: lineNo, Severity: Error, Message: fmt.Sprintf("illegal key %q", key)})
		}
		if val == "" {
			flush(lineNo)
			continue
		}
		if val[0] == '"' || val[0] == '\'' {
			q := val[0]
			if quoteClosed(val[1:], q) {
				flush(lineNo)
				continue
			}
			inQuote = q
			startLine = lineNo
			continue
		}
		if strings.ContainsAny(val, " \t") {
			issues = append(issues, Issue{Line: lineNo, Severity: Warning, Message: "unquoted value contains whitespace"})
		}
		flush(lineNo)
	}
	if inQuote != 0 {
		issues = append(issues, Issue{Line: startLine, Severity: Error, Message: "unterminated quote"})
	}
	return issues
}

func quoteClosed(s string, q byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == q {
			return true
		}
	}
	return false
}
