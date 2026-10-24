package ignore

import (
	"encoding/binary"
	"fmt"
	"os"
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
// Inside a git repository this reads the ignore files and the index directly.
// Otherwise it reads .gitignore files above the project.
func Check(projectPath string) (Status, error) {
	if st, ok := checkWithGit(projectPath); ok {
		return st, nil
	}
	return checkFile(projectPath), nil
}

func checkWithGit(projectPath string) (Status, bool) {
	root, rel, ok := findRepo(projectPath)
	if !ok {
		return Status{}, false
	}
	if indexTracks(root, rel) {
		return Status{
			Code:   "tracked",
			Detail: ".env is tracked by git. A gitignore rule will not apply until it is untracked.",
		}, true
	}
	if ignored, _ := ignoredInRepo(root, rel); ignored {
		return Status{Code: "ignored", Detail: ".env is ignored by git"}, true
	}
	if !hasGitignore(projectPath, root) {
		return Status{Code: "unignored", Detail: "no .gitignore in the project, and .env is not ignored"}, true
	}
	return Status{Code: "unignored", Detail: ".env is not covered by .gitignore"}, true
}

// ignoredInRepo applies exclude files in git's order: the global excludes
// file, .git/info/exclude, then .gitignore from the repository root down to
// the .env file. A later match wins.
func ignoredInRepo(root, rel string) (ignored bool, saw bool) {
	apply := func(base string, data []byte) {
		fileRel, err := filepath.Rel(base, filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return
		}
		saw = true
		for _, line := range strings.Split(string(data), "\n") {
			neg, pattern, ok := parsePattern(line)
			if !ok {
				continue
			}
			if patMatch(pattern, filepath.ToSlash(fileRel)) {
				ignored = !neg
			}
		}
	}
	if path := globalExcludesPath(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			apply(root, data)
		}
	}
	gitDir := gitDirectory(root)
	if data, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude")); err == nil {
		apply(root, data)
	}
	dir := root
	applyFile := func(directory string) {
		data, err := os.ReadFile(filepath.Join(directory, ".gitignore"))
		if err != nil {
			return
		}
		apply(directory, data)
	}
	applyFile(dir)
	for _, part := range strings.Split(rel, "/") {
		if part == ".env" || part == "" {
			break
		}
		dir = filepath.Join(dir, part)
		applyFile(dir)
	}
	return ignored, saw
}

func globalExcludesPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		path := filepath.Join(xdg, "git", "ignore")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".config", "git", "ignore")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func gitDirectory(root string) string {
	path := filepath.Join(root, ".git")
	info, err := os.Stat(path)
	if err != nil {
		return path
	}
	if info.IsDir() {
		return path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return path
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir: "
	if !strings.HasPrefix(line, prefix) {
		return path
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return gitDir
}

// indexTracks reports whether rel is present in the git index. It understands
// index versions 2 and 3. A newer index is treated as untracked.
func indexTracks(root, rel string) bool {
	data, err := os.ReadFile(filepath.Join(gitDirectory(root), "index"))
	if err != nil || len(data) < 12 || string(data[:4]) != "DIRC" {
		return false
	}
	version := binary.BigEndian.Uint32(data[4:8])
	count := binary.BigEndian.Uint32(data[8:12])
	if version < 2 || version > 3 {
		return false
	}
	off := 12
	for i := uint32(0); i < count; i++ {
		if off+62 > len(data) {
			return false
		}
		nameAt := off + 62
		if version == 3 {
			nameAt += 2
		}
		if nameAt >= len(data) {
			return false
		}
		nameEnd := nameAt
		for nameEnd < len(data) && data[nameEnd] != 0 {
			nameEnd++
		}
		if string(data[nameAt:nameEnd]) == rel {
			return true
		}
		entry := nameEnd + 1 - off
		if pad := entry % 8; pad != 0 {
			entry += 8 - pad
		}
		off += entry
	}
	return false
}

// findRepo locates the git repository that contains projectPath and the
// repository-relative path of its .env. A monorepo package has no .git of its own.
func findRepo(projectPath string) (root, rel string, ok bool) {
	dir := projectPath
	for {
		info, err := os.Stat(filepath.Join(dir, ".git"))
		if err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			rel, err = filepath.Rel(dir, filepath.Join(projectPath, ".env"))
			if err != nil {
				return "", "", false
			}
			return dir, filepath.ToSlash(rel), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}

func hasGitignore(from, root string) bool {
	dir := from
	for {
		if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err == nil {
			return true
		}
		if dir == root {
			return false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func checkFile(projectPath string) Status {
	dirs := ancestorDirs(projectPath, 3)
	ignored := false
	sawFile := false
	for i := len(dirs) - 1; i >= 0; i-- {
		rel, err := filepath.Rel(dirs[i], filepath.Join(projectPath, ".env"))
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dirs[i], ".gitignore"))
		if err != nil {
			continue
		}
		sawFile = true
		for _, line := range strings.Split(string(data), "\n") {
			neg, pattern, ok := parsePattern(line)
			if !ok {
				continue
			}
			if patMatch(pattern, filepath.ToSlash(rel)) {
				ignored = !neg
			}
		}
	}
	if ignored {
		return Status{Code: "ignored", Detail: ".env is ignored by .gitignore"}
	}
	if !sawFile {
		return Status{Code: "unignored", Detail: "no .gitignore in the project, and .env is not ignored"}
	}
	return Status{Code: "unignored", Detail: ".env is not covered by .gitignore"}
}

func ancestorDirs(start string, extra int) []string {
	dirs := []string{start}
	dir := start
	for i := 0; i < extra; i++ {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dirs = append(dirs, parent)
		dir = parent
	}
	return dirs
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
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	rel = filepath.ToSlash(rel)
	if anchored || strings.Contains(pattern, "/") {
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

// IgnoreRule is the line added when .env is not covered.
func IgnoreRule() string {
	return ".env\n"
}

// AddEnvIgnore appends a .env rule to the project root .gitignore, creating
// the file when it is missing. It does nothing when a rule already covers .env.
func AddEnvIgnore(projectPath string) error {
	path := filepath.Join(projectPath, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if envRuleCovers(string(data)) {
		return nil
	}
	var b strings.Builder
	b.Write(data)
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(IgnoreRule())
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func envRuleCovers(text string) bool {
	covered := false
	for _, line := range strings.Split(text, "\n") {
		neg, pattern, ok := parsePattern(line)
		if !ok {
			continue
		}
		if patMatch(pattern, ".env") {
			covered = !neg
		}
	}
	return covered
}

// Describe formats a status for logs.
func Describe(projectPath string) string {
	st, err := Check(projectPath)
	if err != nil {
		return fmt.Sprintf("%s: %v", projectPath, err)
	}
	return st.Message(projectPath)
}
