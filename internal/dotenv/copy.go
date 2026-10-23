package dotenv

import (
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Plain returns the env text unchanged.
func Plain(content string) string {
	return content
}

// Markdown wraps content in a fenced env block for Microsoft Teams.
// The fence is three backticks, or longer when the content itself contains
// a backtick run that would close a shorter fence.
func Markdown(content string) string {
	n := 3
	run := 0
	for _, r := range content {
		if r == '`' {
			run++
			if run >= n {
				n = run + 1
			}
			continue
		}
		run = 0
	}
	fence := strings.Repeat("`", n)
	body := content
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return fence + "env\n" + body + fence + "\n"
}

// Diff is a unified diff from before to after.
func Diff(beforeName, afterName, before, after string) (string, error) {
	return difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(before),
		B:        difflib.SplitLines(after),
		FromFile: beforeName,
		ToFile:   afterName,
		Context:  3,
	})
}
