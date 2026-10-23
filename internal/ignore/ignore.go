package ignore

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Status describes whether a project's root .env is ignored by git.
type Status struct {
	Code   string
	Detail string
}

// NeedsAttention reports that .env is not safely ignored.
func (s Status) NeedsAttention() bool {
	return s.Code != "" && s.Code != "ignored"
}

// Key identifies a notification-worthy state.
func (s Status) Key() string {
	return s.Code + "\n" + s.Detail
}

// Message is the user-facing sentence for a project name.
func (s Status) Message(project string) string {
	return project + ": " + s.Detail
}

// Check reports whether projectPath/.env is covered by gitignore rules.
// Inside a git repository this uses git itself, including global excludes.
// Otherwise it reads the project root .gitignore.
func Check(projectPath string) (Status, error) {
	if st, ok := checkWithGit(projectPath); ok {
		return st, nil
	}
	return checkFile(projectPath), nil
}

func checkWithGit(projectPath string) (Status, bool) {
	if _, err := os.Stat(filepath.Join(projectPath, ".git")); err != nil {
		return Status{}, false
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Status{}, false
	}
	cmd := exec.Command("git", "-C", projectPath, "check-ignore", "-q", "--", ".env")
	err := cmd.Run()
	if err == nil {
		return Status{Code: "ignored", Detail: ".env is ignored by git"}, true
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return Status{}, false
	}
	tracked := exec.Command("git", "-C", projectPath, "ls-files", "--error-unmatch", "--", ".env")
	tracked.Stdout = nil
	tracked.Stderr = nil
	if err := tracked.Run(); err == nil {
		return Status{
			Code:   "tracked",
			Detail: ".env is tracked by git. A gitignore rule will not apply until it is untracked.",
		}, true
	}
	if _, err := os.Stat(filepath.Join(projectPath, ".gitignore")); err != nil {
		return Status{Code: "unignored", Detail: "no .gitignore in the project, and .env is not ignored"}, true
	}
	return Status{Code: "unignored", Detail: ".env is not covered by .gitignore"}, true
}

func checkFile(projectPath string) Status {
	data, err := os.ReadFile(filepath.Join(projectPath, ".gitignore"))
	if err != nil {
		return Status{Code: "unignored", Detail: "no .gitignore in the project, and .env is not ignored"}
	}
	ignored := false
	for _, line := range strings.Split(string(data), "\n") {
		neg, pattern, ok := parsePattern(line)
		if !ok {
			continue
		}
		if patMatch(pattern, ".env") {
			ignored = !neg
		}
	}
	if ignored {
		return Status{Code: "ignored", Detail: ".env is ignored by .gitignore"}
	}
	return Status{Code: "unignored", Detail: ".env is not covered by .gitignore"}
}

func parsePattern(line string) (neg bool, pattern string, ok bool) {
	line = strings.TrimRight(line, "\r")
	line = strings.TrimRight(line, " ")
	if line == "" || strings.HasPrefix(line, "#") {
		return false, "", false
	}
	if strings.HasPrefix(line, "!") {
		neg = true
		line = line[1:]
	}
	if line == "" {
		return false, "", false
	}
	return neg, line, true
}

func patMatch(pattern, rel string) bool {
	if strings.HasSuffix(pattern, "/") {
		return false
	}
	pattern = strings.TrimPrefix(pattern, "/")
	if strings.Contains(pattern, "/") {
		return wildMatch(pattern, rel)
	}
	return wildMatch(pattern, filepath.Base(rel))
}

func wildMatch(pat, str string) bool {
	for {
		if pat == "" {
			return str == ""
		}
		if strings.HasPrefix(pat, "**") {
			rest := strings.TrimPrefix(pat, "**")
			rest = strings.TrimPrefix(rest, "/")
			if rest == "" {
				return true
			}
			for i := 0; i <= len(str); i++ {
				if wildMatch(rest, str[i:]) {
					return true
				}
			}
			return false
		}
		if pat[0] == '*' {
			rest := pat[1:]
			if wildMatch(rest, str) {
				return true
			}
			for i := 0; i < len(str) && str[i] != '/'; i++ {
				if wildMatch(rest, str[i+1:]) {
					return true
				}
			}
			return false
		}
		if len(str) == 0 {
			return false
		}
		if pat[0] == '?' {
			if str[0] == '/' {
				return false
			}
			pat = pat[1:]
			str = str[1:]
			continue
		}
		if pat[0] != str[0] {
			return false
		}
		pat = pat[1:]
		str = str[1:]
	}
}

// IgnoreRule is the line suggested when .env is not covered.
func IgnoreRule() string {
	return ".env\n"
}

// Describe formats a status for logs.
func Describe(projectPath string) string {
	st, err := Check(projectPath)
	if err != nil {
		return fmt.Sprintf("%s: %v", projectPath, err)
	}
	return st.Message(projectPath)
}
