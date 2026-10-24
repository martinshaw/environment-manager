package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Canonical returns a cleaned absolute path. Missing paths are still cleaned
// so a project can point at a directory that is about to be created.
func Canonical(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if _, statErr := os.Stat(abs); os.IsNotExist(statErr) || os.IsNotExist(err) {
			return filepath.Clean(abs), nil
		}
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// Scan refreshes projects from the configured roots.
func (s *Store) Scan() error {
	_, err := s.withData(func(d *Data) error {
		return mergeScan(d)
	})
	return err
}

// AddRoot appends a directory root.
func (s *Store) AddRoot(path string) error {
	_, err := s.withData(func(d *Data) error {
		return addRoot(d, path)
	})
	return err
}

// RemoveRoot drops a directory root. Discovered projects are kept.
func (s *Store) RemoveRoot(path string) error {
	_, err := s.withData(func(d *Data) error {
		cp, err := Canonical(path)
		if err != nil {
			return err
		}
		var roots []string
		found := false
		for _, root := range d.Config.Roots {
			if root == cp {
				found = true
				continue
			}
			roots = append(roots, root)
		}
		if !found {
			return fmt.Errorf("root not found: %s", cp)
		}
		if roots == nil {
			roots = []string{}
		}
		d.Config.Roots = roots
		return nil
	})
	return err
}

func addRoot(d *Data, path string) error {
	cp, err := Canonical(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(cp)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", cp)
	}
	for _, root := range d.Config.Roots {
		if root == cp {
			return fmt.Errorf("%s is already a root", cp)
		}
	}
	d.Config.Roots = append(d.Config.Roots, cp)
	return nil
}

func mergeScan(d *Data) error {
	seen := map[string]bool{}
	for _, root := range d.Config.Roots {
		rootPath, err := Canonical(root)
		if err != nil {
			continue
		}
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			cp, err := Canonical(filepath.Join(rootPath, name))
			if err != nil {
				continue
			}
			seen[cp] = true
			if idx := indexByPath(d.Projects, cp); idx >= 0 {
				d.Projects[idx].Missing = false
			} else {
				d.Projects = append(d.Projects, Project{
					ID:   uuid.NewString(),
					Name: name,
					Path: cp,
				})
			}
			for _, envDir := range nestedEnvDirs(cp) {
				nested, err := Canonical(envDir)
				if err != nil || nested == cp {
					continue
				}
				seen[nested] = true
				if idx := indexByPath(d.Projects, nested); idx >= 0 {
					d.Projects[idx].Missing = false
					continue
				}
				rel, err := filepath.Rel(rootPath, nested)
				if err != nil || rel == "." {
					rel = filepath.Base(nested)
				}
				d.Projects = append(d.Projects, Project{
					ID:   uuid.NewString(),
					Name: filepath.ToSlash(rel),
					Path: nested,
				})
			}
		}
	}
	for i := range d.Projects {
		if seen[d.Projects[i].Path] {
			d.Projects[i].Missing = false
			continue
		}
		if _, err := os.Stat(d.Projects[i].Path); err != nil {
			d.Projects[i].Missing = true
			continue
		}
		d.Projects[i].Missing = false
	}
	return nil
}

const envSearchDepth = 5

func nestedEnvDirs(projectPath string) []string {
	var found []string
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			if entry.IsDir() {
				if depth >= envSearchDepth || skipNestedDir(name) {
					continue
				}
				walk(filepath.Join(dir, name), depth+1)
				continue
			}
			if name == ".env" && entry.Type().IsRegular() {
				found = append(found, dir)
			}
		}
	}
	walk(projectPath, 0)
	return found
}

func skipNestedDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "dist", "build", "out", "coverage", "target", "Pods", "bower_components", "venv", "__pycache__":
		return true
	default:
		return false
	}
}

func indexByPath(projects []Project, path string) int {
	for i := range projects {
		if projects[i].Path == path {
			return i
		}
	}
	return -1
}

// TerminalDir is the directory the terminal is in, which is what `pwd` prints.
// A PWD value that points at a different directory is ignored.
func TerminalDir() (string, error) {
	dot, err := os.Stat(".")
	if err != nil {
		return os.Getwd()
	}
	if pwd := os.Getenv("PWD"); pwd != "" && filepath.IsAbs(pwd) {
		info, statErr := os.Stat(pwd)
		if statErr == nil && os.SameFile(dot, info) {
			return filepath.Clean(pwd), nil
		}
	}
	return os.Getwd()
}

// Resolve returns the project whose path is the terminal directory or the nearest ancestor.
// Configured roots themselves are not projects unless they were saved as one.
func Resolve(projects []Project, cwd string) (string, error) {
	got, err := Canonical(cwd)
	if err != nil {
		return "", err
	}
	bestID := ""
	bestLen := -1
	for _, p := range projects {
		if p.Missing {
			continue
		}
		pp := filepath.Clean(p.Path)
		if got == pp || strings.HasPrefix(got, pp+string(os.PathSeparator)) {
			if len(pp) > bestLen {
				bestLen = len(pp)
				bestID = p.ID
			}
		}
	}
	if bestID == "" {
		return "", ErrNoProject
	}
	return bestID, nil
}
