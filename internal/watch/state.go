package watch

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"martinshaw.co/environment-manager/internal/pending"
)

type projectState struct {
	NotifiedDriftHash string `json:"notifiedDriftHash,omitempty"`
	NotifiedIgnore    string `json:"notifiedIgnore,omitempty"`
	PauseDrift        bool   `json:"pauseDrift,omitempty"`
}

type stateFile struct {
	Projects map[string]projectState `json:"projects"`
}

func statePath(dir string) string {
	return filepath.Join(dir, "state.json")
}

func withLock(dir string, fn func() error) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(dir, "watch.lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	return fn()
}

func loadState(dir string) (stateFile, error) {
	b, err := os.ReadFile(statePath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return stateFile{Projects: map[string]projectState{}}, nil
		}
		return stateFile{}, err
	}
	var st stateFile
	if len(b) == 0 {
		return stateFile{Projects: map[string]projectState{}}, nil
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return stateFile{}, err
	}
	if st.Projects == nil {
		st.Projects = map[string]projectState{}
	}
	return st, nil
}

// SuppressDrift records hashes that were just stored and drops those drift
// findings so a check that observed the old metadata cannot bring them back.
func SuppressDrift(dir string, hashes map[string]string) error {
	if len(hashes) == 0 {
		return nil
	}
	return withLock(dir, func() error {
		st, err := loadState(dir)
		if err != nil {
			return err
		}
		events, err := pending.Load(dir)
		if err != nil {
			return err
		}
		for id, hash := range hashes {
			ps := st.Projects[id]
			ps.NotifiedDriftHash = hash
			st.Projects[id] = ps
			events = pending.RemoveKind(events, id, pending.KindDrift)
		}
		if err := saveState(dir, st); err != nil {
			return err
		}
		return pending.Save(dir, events)
	})
}

// SuppressIgnore records the gitignore state just handled and drops that finding.
func SuppressIgnore(dir, projectID, statusKey string) error {
	return withLock(dir, func() error {
		st, err := loadState(dir)
		if err != nil {
			return err
		}
		events, err := pending.Load(dir)
		if err != nil {
			return err
		}
		ps := st.Projects[projectID]
		ps.NotifiedIgnore = statusKey
		st.Projects[projectID] = ps
		events = pending.RemoveKind(events, projectID, pending.KindGitignore)
		if err := saveState(dir, st); err != nil {
			return err
		}
		return pending.Save(dir, events)
	})
}

// PauseDrift stops drift findings for a project while an editor has .env open.
func PauseDrift(dir, projectID string) error {
	return withLock(dir, func() error {
		st, err := loadState(dir)
		if err != nil {
			return err
		}
		ps := st.Projects[projectID]
		ps.PauseDrift = true
		st.Projects[projectID] = ps
		return saveState(dir, st)
	})
}

// ResumeDrift allows drift findings for a project again.
func ResumeDrift(dir, projectID string) error {
	return withLock(dir, func() error {
		st, err := loadState(dir)
		if err != nil {
			return err
		}
		ps := st.Projects[projectID]
		if !ps.PauseDrift {
			return nil
		}
		ps.PauseDrift = false
		st.Projects[projectID] = ps
		return saveState(dir, st)
	})
}

// ClearDriftPauses resumes every paused project. A crashed editor session
// must not hide drift after envm exits.
func ClearDriftPauses(dir string) error {
	return withLock(dir, func() error {
		st, err := loadState(dir)
		if err != nil {
			return err
		}
		changed := false
		for id, ps := range st.Projects {
			if !ps.PauseDrift {
				continue
			}
			ps.PauseDrift = false
			st.Projects[id] = ps
			changed = true
		}
		if !changed {
			return nil
		}
		return saveState(dir, st)
	})
}

func saveState(dir string, st stateFile) error {
	if st.Projects == nil {
		st.Projects = map[string]projectState{}
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(statePath(dir), b, 0o600)
}
