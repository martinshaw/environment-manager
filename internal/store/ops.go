package store

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"martinshaw.co/environment-manager/internal/dotenv"
)

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name cannot be empty")
	}
	if strings.ContainsAny(name, "\r\n") {
		return "", fmt.Errorf("name cannot contain newlines")
	}
	return name, nil
}

// UpdateFromDisk stores a new version of the selected env when .env differs
// from the latest snapshot. created is false when the bytes already match.
func (s *Store) UpdateFromDisk(projectID string) (envName string, version int, created bool, err error) {
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		env := p.Selected()
		if env == nil {
			return ErrNoSelected
		}
		content, ok, err := ReadDisk(EnvPath(p.Path))
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoDisk
		}
		v, made, err := s.addVersion(env, content)
		if err != nil {
			return err
		}
		envName = env.Name
		version = v.N
		created = made
		return nil
	})
	return envName, version, created, err
}

// NewFromDisk stores the on-disk .env as a new named env at v1 and selects it.
func (s *Store) NewFromDisk(projectID, name string) (version int, err error) {
	name, err = cleanName(name)
	if err != nil {
		return 0, err
	}
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		if p.EnvByName(name) != nil {
			return ErrDuplicate
		}
		content, ok, err := ReadDisk(EnvPath(p.Path))
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoDisk
		}
		env := EnvFile{ID: uuid.NewString(), Name: name}
		v, _, err := s.addVersion(&env, content)
		if err != nil {
			return err
		}
		p.Envs = append(p.Envs, env)
		p.SelectedEnvID = env.ID
		version = v.N
		return nil
	})
	return version, err
}

// Use writes the latest snapshot of the named env to .env and selects it.
func (s *Store) Use(projectID, name string) (version int, err error) {
	name, err = cleanName(name)
	if err != nil {
		return 0, err
	}
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		env := p.EnvByName(name)
		if env == nil {
			return ErrNoEnv
		}
		lat := env.Latest()
		if lat == nil {
			return fmt.Errorf("env file has no versions")
		}
		plain, err := s.ReadBlob(lat.Blob)
		if err != nil {
			return err
		}
		if err := WriteDisk(EnvPath(p.Path), plain); err != nil {
			return err
		}
		p.SelectedEnvID = env.ID
		version = lat.N
		return nil
	})
	return version, err
}

// Revert writes the selected env's latest snapshot over .env.
func (s *Store) Revert(projectID string) (envName string, version int, err error) {
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		env := p.Selected()
		if env == nil {
			return ErrNoSelected
		}
		lat := env.Latest()
		if lat == nil {
			return fmt.Errorf("env file has no versions")
		}
		plain, err := s.ReadBlob(lat.Blob)
		if err != nil {
			return err
		}
		if err := WriteDisk(EnvPath(p.Path), plain); err != nil {
			return err
		}
		envName = env.Name
		version = lat.N
		return nil
	})
	return envName, version, err
}

// SaveContent stores content as the next version when it differs from the
// latest snapshot. When writeDisk is set and the env is selected, .env is updated.
func (s *Store) SaveContent(projectID, envID string, content []byte, writeDisk bool) (version int, created bool, err error) {
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		env := p.Env(envID)
		if env == nil {
			return ErrNotFound
		}
		v, made, err := s.addVersion(env, content)
		if err != nil {
			return err
		}
		version = v.N
		created = made
		if writeDisk && made && p.SelectedEnvID == env.ID {
			if err := WriteDisk(EnvPath(p.Path), content); err != nil {
				return err
			}
		}
		return nil
	})
	return version, created, err
}

// DeleteVersion removes one snapshot. The last remaining version is kept;
// delete the env file instead.
func (s *Store) DeleteVersion(projectID, envID string, n int) error {
	var blobs []string
	_, err := s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		env := p.Env(envID)
		if env == nil {
			return ErrNotFound
		}
		if len(env.Versions) <= 1 {
			return ErrLastVersion
		}
		keep := make([]Version, 0, len(env.Versions))
		found := false
		for _, v := range env.Versions {
			if v.N == n {
				blobs = append(blobs, v.Blob)
				found = true
				continue
			}
			keep = append(keep, v)
		}
		if !found {
			return ErrNotFound
		}
		env.Versions = keep
		return nil
	})
	if err == nil {
		s.removeBlobs(blobs)
	}
	return err
}

// DeleteEnv removes a stored env file and its snapshots. The on-disk .env is kept.
func (s *Store) DeleteEnv(projectID, envID string) error {
	var blobs []string
	_, err := s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		keep := make([]EnvFile, 0, len(p.Envs))
		found := false
		for _, env := range p.Envs {
			if env.ID == envID {
				found = true
				for _, v := range env.Versions {
					blobs = append(blobs, v.Blob)
				}
				continue
			}
			keep = append(keep, env)
		}
		if !found {
			return ErrNotFound
		}
		p.Envs = keep
		if p.SelectedEnvID == envID {
			p.SelectedEnvID = ""
		}
		return nil
	})
	if err == nil {
		s.removeBlobs(blobs)
	}
	return err
}

// DeleteProject removes the project from the index. The directory is kept.
func (s *Store) DeleteProject(projectID string) error {
	var blobs []string
	_, err := s.withData(func(d *Data) error {
		keep := make([]Project, 0, len(d.Projects))
		found := false
		for _, p := range d.Projects {
			if p.ID == projectID {
				found = true
				for _, env := range p.Envs {
					for _, v := range env.Versions {
						blobs = append(blobs, v.Blob)
					}
				}
				continue
			}
			keep = append(keep, p)
		}
		if !found {
			return ErrNotFound
		}
		d.Projects = keep
		return nil
	})
	if err == nil {
		s.removeBlobs(blobs)
	}
	return err
}

// RenameProject changes the display name.
func (s *Store) RenameProject(projectID, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		p.Name = name
		return nil
	})
	return err
}

// RenameEnv changes a stored env file's name.
func (s *Store) RenameEnv(projectID, envID, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		if other := p.EnvByName(name); other != nil && other.ID != envID {
			return ErrDuplicate
		}
		env := p.Env(envID)
		if env == nil {
			return ErrNotFound
		}
		env.Name = name
		return nil
	})
	return err
}

// SetPath points a project at a different directory.
func (s *Store) SetPath(projectID, path string) (missing bool, err error) {
	_, err = s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		cp, err := Canonical(path)
		if err != nil {
			return err
		}
		for _, other := range d.Projects {
			if other.ID != projectID && other.Path == cp {
				return fmt.Errorf("%s is already a project", cp)
			}
		}
		info, statErr := osStat(cp)
		p.Path = cp
		p.Missing = statErr != nil || info == nil || !info.isDir
		missing = p.Missing
		return nil
	})
	return missing, err
}

type statInfo struct{ isDir bool }

func osStat(path string) (*statInfo, error) {
	info, err := statPath(path)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// SetHidden hides or shows a project in the TUI. Hidden projects still resolve from their directory.
func (s *Store) SetHidden(projectID string, hidden bool) error {
	_, err := s.withData(func(d *Data) error {
		p := d.Project(projectID)
		if p == nil {
			return ErrNotFound
		}
		p.Hidden = hidden
		return nil
	})
	return err
}

// DiskDiff is a unified diff from the selected latest snapshot to the on-disk .env.
func (s *Store) DiskDiff(projectID string) (string, error) {
	d, err := s.View()
	if err != nil {
		return "", err
	}
	p := d.Project(projectID)
	if p == nil {
		return "", ErrNotFound
	}
	env := p.Selected()
	if env == nil {
		return "", ErrNoSelected
	}
	lat := env.Latest()
	if lat == nil {
		return "", fmt.Errorf("env file has no versions")
	}
	stored, err := s.ReadBlob(lat.Blob)
	if err != nil {
		return "", err
	}
	disk, ok, err := ReadDisk(EnvPath(p.Path))
	if err != nil {
		return "", err
	}
	diskName := ".env"
	if !ok {
		disk = nil
		diskName = ".env (missing)"
	}
	return dotenv.Diff(fmt.Sprintf("%s v%d", env.Name, lat.N), diskName, string(stored), string(disk))
}
