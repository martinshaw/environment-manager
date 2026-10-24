package watch

import (
	"fmt"
	"os"

	"martinshaw.co/environment-manager/internal/ignore"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
)

// Hooks receives watcher results. Notify is called only for a new finding.
// Updated is called after the pending queue is saved.
type Hooks struct {
	Notify  func(title, body string)
	Updated func(events []pending.Event)
}

type note struct {
	title string
	body  string
}

// Check scans projects, compares .env bytes to the selected snapshot, and
// checks gitignore coverage. Secret values are not included in notifications.
func Check(dir string, hooks Hooks) error {
	s, err := store.Open(dir)
	if err != nil {
		return err
	}
	if err := s.Scan(); err != nil {
		return err
	}
	var notes []note
	var events []pending.Event
	err = withLock(dir, func() error {
		data, err := store.ReadData(dir)
		if err != nil {
			return err
		}
		st, err := loadState(dir)
		if err != nil {
			return err
		}
		events, err = pending.Load(dir)
		if err != nil {
			return err
		}
		alive := map[string]bool{}
		for _, p := range data.Projects {
			if p.Missing {
				continue
			}
			alive[p.ID] = true
			ps := st.Projects[p.ID]
			latest := ""
			if env := p.Selected(); env != nil {
				if v := env.Latest(); v != nil {
					latest = v.SHA256
				}
			}
			disk, err := store.HashDisk(store.EnvPath(p.Path))
			if err != nil {
				disk = store.MissingHash
			}
			notifyDrift, clearDrift := DecideDrift(latest, disk, ps.NotifiedDriftHash)
			if clearDrift {
				ps.NotifiedDriftHash = ""
				events = pending.RemoveKind(events, p.ID, pending.KindDrift)
			} else if notifyDrift {
				ps.NotifiedDriftHash = disk
				msg := driftMessage(p.Name, latest == "")
				events = pending.Upsert(events, pending.Event{
					Kind:      pending.KindDrift,
					ProjectID: p.ID,
					Project:   p.Name,
					Path:      p.Path,
					Message:   msg,
				})
				notes = append(notes, note{title: "envm", body: msg})
			}
			if !envOnDisk(p.Path) {
				ps.NotifiedIgnore = ""
				events = pending.RemoveKind(events, p.ID, pending.KindGitignore)
			} else {
				ig, err := ignore.Check(p.Path)
				if err != nil {
					st.Projects[p.ID] = ps
					continue
				}
				notifyIgnore, clearIgnore := DecideIgnore(ig.Key(), ps.NotifiedIgnore, ig.NeedsAttention())
				if clearIgnore {
					ps.NotifiedIgnore = ""
					events = pending.RemoveKind(events, p.ID, pending.KindGitignore)
				} else if notifyIgnore {
					ps.NotifiedIgnore = ig.Key()
					msg := ig.Message(p.Name)
					events = pending.Upsert(events, pending.Event{
						Kind:      pending.KindGitignore,
						ProjectID: p.ID,
						Project:   p.Name,
						Path:      p.Path,
						Message:   msg,
					})
					notes = append(notes, note{title: "envm", body: msg})
				}
			}
			st.Projects[p.ID] = ps
		}
		kept := make([]pending.Event, 0, len(events))
		for _, ev := range events {
			if alive[ev.ProjectID] {
				kept = append(kept, ev)
			}
		}
		events = kept
		if err := saveState(dir, st); err != nil {
			return err
		}
		return pending.Save(dir, events)
	})
	if err != nil {
		return err
	}
	if hooks.Notify != nil {
		for _, n := range notes {
			hooks.Notify(n.title, n.body)
		}
	}
	if hooks.Updated != nil {
		hooks.Updated(events)
	}
	return nil
}

// envOnDisk reports whether the project directory contains a .env file.
func envOnDisk(projectPath string) bool {
	info, err := os.Stat(store.EnvPath(projectPath))
	return err == nil && info.Mode().IsRegular()
}

// OmitGitignoreWithoutEnv drops gitignore findings for projects that have no .env file.
func OmitGitignoreWithoutEnv(events []pending.Event) []pending.Event {
	out := make([]pending.Event, 0, len(events))
	for _, ev := range events {
		if ev.Kind == pending.KindGitignore && !envOnDisk(ev.Path) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func driftMessage(project string, unsaved bool) string {
	if unsaved {
		return fmt.Sprintf("%s: .env is not stored yet. Save it as a new file.", project)
	}
	return fmt.Sprintf("%s: .env changed. Update the selected file, save it as a new file, or revert.", project)
}

// Dismiss removes one pending event. The finding is not raised again until
// the disk hash or gitignore status changes.
func Dismiss(dir, id string) error {
	return withLock(dir, func() error {
		events, err := pending.Load(dir)
		if err != nil {
			return err
		}
		out := make([]pending.Event, 0, len(events))
		for _, ev := range events {
			if ev.ID == id {
				continue
			}
			out = append(out, ev)
		}
		return pending.Save(dir, out)
	})
}
