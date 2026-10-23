package pending

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Event is a watcher finding. It carries project names, never secret values.
type Event struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	ProjectID string    `json:"projectId"`
	Project   string    `json:"project"`
	Path      string    `json:"path"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

const (
	KindDrift     = "drift"
	KindGitignore = "gitignore"
)

type file struct {
	Events []Event `json:"events"`
}

func path(dir string) string {
	return filepath.Join(dir, "pending.json")
}

// Load reads the pending queue. A missing file is an empty queue.
func Load(dir string) ([]Event, error) {
	b, err := os.ReadFile(path(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f file
	if len(b) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return f.Events, nil
}

// Save writes events with mode 0600.
func Save(dir string, events []Event) error {
	if events == nil {
		events = []Event{}
	}
	b, err := json.MarshalIndent(file{Events: events}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeAtomic(path(dir), b)
}

// Upsert replaces an existing event for the same project and kind.
func Upsert(events []Event, e Event) []Event {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	out := make([]Event, 0, len(events)+1)
	replaced := false
	for _, ev := range events {
		if ev.ProjectID == e.ProjectID && ev.Kind == e.Kind {
			if !replaced {
				out = append(out, e)
				replaced = true
			}
			continue
		}
		out = append(out, ev)
	}
	if !replaced {
		out = append(out, e)
	}
	return out
}

// RemoveKind drops events of kind for a project.
func RemoveKind(events []Event, projectID, kind string) []Event {
	out := make([]Event, 0, len(events))
	for _, ev := range events {
		if ev.ProjectID == projectID && ev.Kind == kind {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// Dismiss removes one event by id.
func Dismiss(dir, id string) error {
	events, err := Load(dir)
	if err != nil {
		return err
	}
	out := make([]Event, 0, len(events))
	for _, ev := range events {
		if ev.ID == id {
			continue
		}
		out = append(out, ev)
	}
	return Save(dir, out)
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pending-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(name)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return err
		}
		if err := os.Rename(name, path); err != nil {
			return err
		}
	}
	cleanup = false
	return nil
}
