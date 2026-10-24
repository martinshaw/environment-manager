package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"martinshaw.co/environment-manager/internal/crypto"
	"martinshaw.co/environment-manager/internal/daemon"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
	"martinshaw.co/environment-manager/internal/tui"
	"martinshaw.co/environment-manager/internal/upgrade"
	"martinshaw.co/environment-manager/internal/watch"
)

// Execute runs the envm command line.
func Execute(version string) error {
	return newRoot(version).Execute()
}

func newRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "envm",
		Short: "Manage versioned .env files for local projects",
		Long: `envm stores named, versioned .env files for projects under the directories you choose.

The TUI searches projects, stored env files, and versions. Commands that read or write .env use the directory the terminal is in (pwd) to find the project.`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(tui.Options{Version: version})
		},
	}
	root.AddCommand(
		useCmd(),
		updateCmd(),
		newCmd(),
		revertCmd(),
		diffCmd(),
		driftCmd(version),
		rootsCmd(),
		daemonCmd(),
		upgradeCmd(version),
		&cobra.Command{
			Use:   "version",
			Short: "Print the envm version",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println(version)
			},
		},
	)
	return root
}

func useCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Write a stored env file to .env and select it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProject(func(s *store.Store, project store.Project) error {
				n, err := s.Use(project.ID, args[0])
				if err != nil {
					return err
				}
				fmt.Printf("%s: selected %s v%d and wrote .env\n", project.Name, args[0], n)
				return nil
			})
		},
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Store a new version of the selected env when .env has changed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProject(func(s *store.Store, project store.Project) error {
				name, n, created, err := s.UpdateFromDisk(project.ID)
				if err != nil {
					return err
				}
				if !created {
					fmt.Printf("%s: .env matches %s v%d\n", project.Name, name, n)
					return nil
				}
				fmt.Printf("%s: stored %s v%d\n", project.Name, name, n)
				return nil
			})
		},
	}
}

func newCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Store the current .env as a new named env file and select it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProject(func(s *store.Store, project store.Project) error {
				n, err := s.NewFromDisk(project.ID, args[0])
				if err != nil {
					return err
				}
				fmt.Printf("%s: stored %s v%d and selected it\n", project.Name, args[0], n)
				return nil
			})
		},
	}
}

func revertCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revert",
		Short: "Restore .env to the latest version of the selected env file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProject(func(s *store.Store, project store.Project) error {
				name, n, err := s.Revert(project.ID)
				if err != nil {
					return err
				}
				fmt.Printf("%s: restored .env to %s v%d\n", project.Name, name, n)
				return nil
			})
		},
	}
}

func diffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show a unified diff from the selected version to the on-disk .env",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withProject(func(s *store.Store, project store.Project) error {
				text, err := s.DiskDiff(project.ID)
				if err != nil {
					return err
				}
				if text == "" {
					fmt.Fprintln(os.Stderr, "No differences.")
					return nil
				}
				fmt.Print(text)
				return nil
			})
		},
	}
}

func driftCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "drift",
		Short: "Review .env changes and gitignore findings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := store.DefaultDir()
			if err != nil {
				return err
			}
			if _, err := store.Open(dir); err != nil {
				return err
			}
			if err := watch.Check(dir, watch.Hooks{}); err != nil {
				return err
			}
			events, err := pending.Load(dir)
			if err != nil {
				return err
			}
			if len(events) == 0 {
				fmt.Println("No pending changes.")
				return nil
			}
			return tui.Run(tui.Options{Version: version})
		},
	}
}

func rootsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roots",
		Short: "Manage the directories whose children are projects",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "add <path>",
			Short: "Add a project root and scan it",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				s, err := openStore()
				if err != nil {
					return err
				}
				if err := s.AddRoot(args[0]); err != nil {
					return err
				}
				if err := s.Scan(); err != nil {
					return err
				}
				fmt.Println("Added root.")
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove <path>",
			Short: "Remove a project root",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				s, err := openStore()
				if err != nil {
					return err
				}
				return s.RemoveRoot(args[0])
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List project roots",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				s, err := openStore()
				if err != nil {
					return err
				}
				data, err := s.View()
				if err != nil {
					return err
				}
				if len(data.Config.Roots) == 0 {
					fmt.Println("No roots. Add one with envm roots add.")
					return nil
				}
				for _, root := range data.Config.Roots {
					n := 0
					for _, p := range data.Projects {
						if p.Path == root || hasPrefixPath(p.Path, root) {
							n++
						}
					}
					fmt.Printf("%s (%d projects)\n", root, n)
				}
				return nil
			},
		},
	)
	return cmd
}

func daemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run or install the .env and .gitignore watcher",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "Run the watcher in the foreground",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := store.DefaultDir()
				if err != nil {
					return err
				}
				ctx, cancel := daemon.WithSignals()
				defer cancel()
				return daemon.Run(ctx, dir)
			},
		},
		&cobra.Command{
			Use:   "install",
			Short: "Install and start the login watcher",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := store.DefaultDir()
				if err != nil {
					return err
				}
				if err := daemon.InstallService(dir); err != nil {
					return err
				}
				fmt.Println("Watcher installed.")
				return nil
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Stop the watcher and remove the login service",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := store.DefaultDir()
				if err != nil {
					return err
				}
				if err := daemon.UninstallService(dir); err != nil {
					return err
				}
				fmt.Println("Watcher removed.")
				return nil
			},
		},
		&cobra.Command{
			Use:   "start",
			Short: "Start the watcher",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := store.DefaultDir()
				if err != nil {
					return err
				}
				if err := daemon.Start(dir); err != nil {
					return err
				}
				fmt.Println(daemon.Status(dir))
				return nil
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the watcher",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := store.DefaultDir()
				if err != nil {
					return err
				}
				if err := daemon.Stop(dir); err != nil {
					return err
				}
				fmt.Println("stopped")
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show whether the watcher is running",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := store.DefaultDir()
				if err != nil {
					return err
				}
				fmt.Println(daemon.Status(dir))
				return nil
			},
		},
	)
	return cmd
}

func upgradeCmd(version string) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Install the latest envm release from GitHub",
		Long: `Download the release archive for this operating system, check it against checksums.txt, and replace the current envm binary.

envm update stores a new version of the selected .env file. envm upgrade replaces the envm program itself.

A running watcher is restarted so it loads the new binary. If the binary moved to a different path, run envm daemon install again.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := upgrade.Run(cmd.Context(), upgrade.Options{
				Current:   version,
				CheckOnly: check,
				Stdout:    cmd.OutOrStdout(),
			})
			if err != nil {
				return err
			}
			if !res.Updated {
				return nil
			}
			dir, err := store.DefaultDir()
			if err != nil || !daemon.Running(dir) {
				return nil
			}
			if err := daemon.Stop(dir); err != nil || daemon.Start(dir) != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "Updated the binary. Restart the watcher with envm daemon start.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Restarted the watcher.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Report the latest release without installing it")
	return cmd
}

func openStore() (*store.Store, error) {
	dir, err := store.DefaultDir()
	if err != nil {
		return nil, err
	}
	return store.Open(dir)
}

func withProject(fn func(*store.Store, store.Project) error) error {
	s, err := openUnlocked()
	if err != nil {
		return err
	}
	cwd, err := store.TerminalDir()
	if err != nil {
		return err
	}
	if err := s.Scan(); err != nil {
		return err
	}
	data, err := s.View()
	if err != nil {
		return err
	}
	id, err := store.Resolve(data.Projects, cwd)
	if err != nil {
		if errors.Is(err, store.ErrNoProject) {
			return fmt.Errorf("no envm project for %s. Add its parent with envm roots add", cwd)
		}
		return err
	}
	p := data.Project(id)
	if p == nil {
		return store.ErrNoProject
	}
	return fn(s, *p)
}

func openUnlocked() (*store.Store, error) {
	s, err := openStore()
	if err != nil {
		return nil, err
	}
	if !store.Initialized(s.Dir()) {
		fmt.Fprintln(os.Stderr, "Create a passphrase to encrypt stored .env files.")
		first, err := readPassphrase("New passphrase: ")
		if err != nil {
			return nil, err
		}
		second, err := readPassphrase("Confirm passphrase: ")
		if err != nil {
			return nil, err
		}
		if first != second {
			return nil, errors.New("passphrases did not match")
		}
		if len(first) < crypto.MinPassphraseLen {
			return nil, fmt.Errorf("passphrase must be at least %d characters", crypto.MinPassphraseLen)
		}
		recovery, err := s.CreateVault(first, 0)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "\nRecovery key, shown once. Store it somewhere safe.\nWithout it, a forgotten passphrase cannot be undone.\n\n%s\n\n", recovery)
		return s, nil
	}
	pass, err := readPassphrase("Passphrase: ")
	if err != nil {
		return nil, err
	}
	if err := s.Unlock(pass); err != nil {
		return nil, err
	}
	return s, nil
}

func readPassphrase(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("a terminal is required to enter the passphrase")
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func hasPrefixPath(path, root string) bool {
	if len(path) <= len(root) || root == "" {
		return false
	}
	return path[:len(root)] == root && (root[len(root)-1] == os.PathSeparator || path[len(root)] == os.PathSeparator)
}
