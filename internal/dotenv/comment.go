package dotenv

import (
	"fmt"
	"strings"
)

// ToggleComment comments or uncomments the 0-based line. A blank line is
// unchanged. The original trailing-newline state is preserved.
func ToggleComment(content string, line int) (string, error) {
	lines, trailing := Lines(content)
	if line < 0 || line >= len(lines) {
		return content, fmt.Errorf("line %d is out of range", line+1)
	}
	next := toggleLine(lines[line])
	if next == lines[line] {
		return content, nil
	}
	lines[line] = next
	return Join(lines, trailing), nil
}

func toggleLine(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		rest := strings.TrimPrefix(trimmed, "#")
		rest = strings.TrimPrefix(rest, " ")
		return indent + rest
	}
	return indent + "# " + trimmed
}
