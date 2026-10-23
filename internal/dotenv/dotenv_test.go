package dotenv

import (
	"strings"
	"testing"
)

func TestLint(t *testing.T) {
	src := strings.Join([]string{
		"APP_KEY=abc",
		"export DB_URL=postgres://localhost/db",
		"# comment",
		"",
		"KEY = value",
		"BAD KEY=x",
		`QUOTE="hello`,
		`world"`,
		"APP_KEY=again",
		"SPACE=hello world",
		"nope",
	}, "\n")
	issues := Lint(src)
	var messages []string
	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"illegal key", "duplicate key APP_KEY", "unquoted value contains whitespace", "missing '='"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "unterminated") {
		t.Errorf("quoted multiline should be valid, got:\n%s", joined)
	}
	if len(Lint("export EMPTY=\n")) != 0 {
		t.Fatal("empty export value should be valid")
	}
	open := Lint("QUOTE=\"abc\n")
	if len(open) != 1 || open[0].Message != "unterminated quote" {
		t.Fatalf("got %#v", open)
	}
}

func TestToggleComment(t *testing.T) {
	src := "APP_KEY=abc\n# OTHER=1\n\n  NAME=ada\n"
	out, err := ToggleComment(src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out != "# APP_KEY=abc\n# OTHER=1\n\n  NAME=ada\n" {
		t.Fatalf("comment: %q", out)
	}
	out, err = ToggleComment(out, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out != src {
		t.Fatalf("round trip: %q", out)
	}
	out, err = ToggleComment(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OTHER=1\n") || strings.Contains(strings.Split(out, "\n")[1], "#") {
		t.Fatalf("uncomment: %q", out)
	}
	blank, err := ToggleComment(src, 2)
	if err != nil {
		t.Fatal(err)
	}
	if blank != src {
		t.Fatalf("blank line changed: %q", blank)
	}
	indented, err := ToggleComment(src, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(indented, "  # NAME=ada\n") {
		t.Fatalf("indent: %q", indented)
	}
	noNL := "A=1"
	out, err = ToggleComment(noNL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out != "# A=1" {
		t.Fatalf("lost no-newline state: %q", out)
	}
}

func TestMarkdownAndDiff(t *testing.T) {
	md := Markdown("A=1\n")
	if md != "```env\nA=1\n```\n" {
		t.Fatalf("markdown: %q", md)
	}
	long := Markdown("```\nA=1\n")
	if !strings.HasPrefix(long, "````env\n") {
		t.Fatalf("longer fence: %q", long)
	}
	diff, err := Diff("v1", "v2", "A=1\n", "A=2\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "--- v1") || !strings.Contains(diff, "+++ v2") || !strings.Contains(diff, "-A=1") {
		t.Fatalf("diff: %q", diff)
	}
	same, err := Diff("v1", "v2", "A=1\n", "A=1\n")
	if err != nil {
		t.Fatal(err)
	}
	if same != "" {
		t.Fatalf("expected empty diff, got %q", same)
	}
}
