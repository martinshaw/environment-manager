package watch

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type projectState struct {
	NotifiedDriftHash string `json:"notifiedDriftHash,omitempty"`
	NotifiedIgnore    string `json:"notifiedIgnore,omitempty"`
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
