package watch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"martinshaw.co/environment-manager/internal/store"
)

// Loop watches project directories until ctx is cancelled.
func Loop(ctx context.Context, dir string, hooks Hooks) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watched := map[string]bool{}
	refresh := func() {
		s, err := store.Open(dir)
		if err != nil {
			return
		}
		data, err := s.View()
		if err != nil {
			return
		}
		want := map[string]bool{}
		for _, root := range data.Config.Roots {
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				want[root] = true
			}
		}
		for _, p := range data.Projects {
			if p.Missing {
				continue
			}
			if info, err := os.Stat(p.Path); err == nil && info.IsDir() {
				want[p.Path] = true
			}
		}
		for path := range watched {
			if !want[path] {
				_ = watcher.Remove(path)
				delete(watched, path)
			}
		}
		for path := range want {
			if watched[path] {
				continue
			}
			if err := watcher.Add(path); err == nil {
				watched[path] = true
			}
		}
	}

	if err := Check(dir, hooks); err != nil {
		logLine(dir, err.Error())
	}
	refresh()

	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	defer debounce.Stop()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := Check(dir, hooks); err != nil {
				logLine(dir, err.Error())
			}
			refresh()
		case _, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(300 * time.Millisecond)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			if err != nil {
				logLine(dir, err.Error())
			}
		case <-debounce.C:
			if err := Check(dir, hooks); err != nil {
				logLine(dir, err.Error())
			}
			refresh()
		}
	}
}

func logLine(dir, msg string) {
	f, err := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), msg)
}
