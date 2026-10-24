package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"martinshaw.co/environment-manager/internal/daemon"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
	"martinshaw.co/environment-manager/internal/watch"
)

// Run starts the TUI. It prompts for the vault passphrase when needed.
func Run(opts Options) error {
	dir := opts.Dir
	if dir == "" {
		var err error
		dir, err = store.DefaultDir()
		if err != nil {
			return err
		}
	}
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = store.TerminalDir()
		if err != nil {
			return err
		}
	}
	st, err := store.Open(dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !daemon.Running(dir) {
		go func() {
			_ = watch.Loop(ctx, dir, watch.Hooks{})
		}()
	}
	m := newModel(dir, cwd, opts.Version, ctx, st)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	_, err = prog.Run()
	return err
}

func listenEvents(ctx context.Context, dir string) []pending.Event {
	if ctx == nil {
		ev, _ := pending.Load(dir)
		return ev
	}
	return daemon.NextEvents(ctx, dir)
}
