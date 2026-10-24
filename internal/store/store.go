package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
	"martinshaw.co/environment-manager/internal/crypto"
)

const (
	configName   = "config.json"
	projectsName = "projects.json"
	contentDir   = "content"
	lockName     = "store.lock"
	// MissingHash is the disk hash used when .env does not exist.
	MissingHash = "missing"
)

var (
	ErrNoProject   = errors.New("no envm project for this directory")
	ErrNoEnv       = errors.New("no stored env file with that name")
	ErrNoSelected  = errors.New("no selected env file")
	ErrNoDisk      = errors.New("no .env file in the project directory")
	ErrDuplicate   = errors.New("an env file with that name already exists")
	ErrNotFound    = errors.New("not found")
	ErrLastVersion = errors.New("delete the env file to remove its only version")
	ErrLocked      = errors.New("vault is locked")
	ErrExists      = errors.New("vault already exists")
)

// DefaultEnvName is the env file name used for a project's .env during first-run setup.
const DefaultEnvName = "Local Development"

// Config is the non-secret root list.
type Config struct {
	Roots     []string `json:"roots"`
	SetupDone bool     `json:"setupDone,omitempty"`
}

// Project is one child directory of a root, or a directory the user pointed at.
type Project struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	SelectedEnvID string    `json:"selectedEnvId,omitempty"`
	Hidden        bool      `json:"hidden,omitempty"`
	Missing       bool      `json:"missing,omitempty"`
	Envs          []EnvFile `json:"envs,omitempty"`
}

// EnvFile is a named stored .env, with immutable snapshots.
type EnvFile struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Versions []Version `json:"versions,omitempty"`
}

// Version is one full snapshot. Blob names a file under content/.
type Version struct {
	N         int       `json:"n"`
	CreatedAt time.Time `json:"createdAt"`
	SHA256    string    `json:"sha256"`
	Blob      string    `json:"blob"`
	Bytes     int       `json:"bytes"`
}

// Data is the on-disk metadata loaded together.
type Data struct {
	Config   Config    `json:"-"`
	Projects []Project `json:"projects"`
}

// Store reads and writes ~/.envm. key is nil until the vault is unlocked.
type Store struct {
	dir  string
	key  []byte
	lock *flock.Flock
}

// DefaultDir is ~/.envm, or ENVM_HOME when that is set.
func DefaultDir() (string, error) {
	if d := os.Getenv("ENVM_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".envm"), nil
}

// Initialized reports whether a vault key exists.
func Initialized(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, crypto.KeyFile))
	return err == nil
}

// Open creates the envm directory if needed. It does not unlock the vault.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, contentDir), 0o700); err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0o700)
	return &Store{dir: dir, lock: flock.New(filepath.Join(dir, lockName))}, nil
}

// Dir is the envm directory.
func (s *Store) Dir() string { return s.dir }

// CreateVault writes a new key file and keeps the data key in memory.
func (s *Store) CreateVault(passphrase string, work int) (string, error) {
	if Initialized(s.dir) {
		return "", ErrExists
	}
	key, recovery, err := crypto.Create(s.dir, passphrase, work)
	if err != nil {
		return "", err
	}
	s.key = key
	return recovery, nil
}

// Unlock opens the vault with a passphrase.
func (s *Store) Unlock(passphrase string) error {
	key, err := crypto.UnlockPassphrase(s.dir, passphrase)
	if err != nil {
		return err
	}
	s.key = key
	return nil
}

// UnlockRecovery opens the vault with the one-time recovery key.
func (s *Store) UnlockRecovery(recovery string) error {
	key, err := crypto.UnlockRecovery(s.dir, recovery)
	if err != nil {
		return err
	}
	s.key = key
	return nil
}

// View loads a snapshot of the metadata.
func (s *Store) View() (Data, error) {
	if err := s.lock.Lock(); err != nil {
		return Data{}, err
	}
	defer s.lock.Unlock()
	return s.load()
}

func (s *Store) withData(fn func(*Data) error) (Data, error) {
	if err := s.lock.Lock(); err != nil {
		return Data{}, err
	}
	defer s.lock.Unlock()
	d, err := s.load()
	if err != nil {
		return Data{}, err
	}
	if err := fn(&d); err != nil {
		return Data{}, err
	}
	if err := s.save(d); err != nil {
		return Data{}, err
	}
	return d, nil
}

func (s *Store) load() (Data, error) {
	var d Data
	cfg, err := readJSON[Config](filepath.Join(s.dir, configName))
	if err != nil {
		return Data{}, err
	}
	if cfg != nil {
		d.Config = *cfg
	}
	projects, err := readJSON[struct {
		Projects []Project `json:"projects"`
	}](filepath.Join(s.dir, projectsName))
	if err != nil {
		return Data{}, err
	}
	if projects != nil {
		d.Projects = projects.Projects
	}
	if d.Config.Roots == nil {
		d.Config.Roots = []string{}
	}
	if d.Projects == nil {
		d.Projects = []Project{}
	}
	return d, nil
}

func (s *Store) save(d Data) error {
	if d.Config.Roots == nil {
		d.Config.Roots = []string{}
	}
	if d.Projects == nil {
		d.Projects = []Project{}
	}
	if err := writeJSON(filepath.Join(s.dir, configName), d.Config); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, projectsName), struct {
		Projects []Project `json:"projects"`
	}{Projects: d.Projects})
}

func readJSON[T any](path string) (*T, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeAtomic(path, b, 0o600)
}

func (s *Store) blobPath(name string) string {
	return filepath.Join(s.dir, contentDir, name)
}

func (s *Store) writeBlob(plain []byte) (string, error) {
	if len(s.key) != 32 {
		return "", ErrLocked
	}
	sealed, err := crypto.Seal(s.key, plain)
	if err != nil {
		return "", err
	}
	name := uuid.NewString()
	if err := writeAtomic(s.blobPath(name), sealed, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

// ReadBlob decrypts a version blob.
func (s *Store) ReadBlob(name string) ([]byte, error) {
	if len(s.key) != 32 {
		return nil, ErrLocked
	}
	if !validBlob(name) {
		return nil, errors.New("invalid blob name")
	}
	b, err := os.ReadFile(s.blobPath(name))
	if err != nil {
		return nil, err
	}
	return crypto.Open(s.key, b)
}

func validBlob(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return filepath.Base(name) == name
}

func (s *Store) removeBlobs(names []string) {
	for _, name := range names {
		if validBlob(name) {
			_ = os.Remove(s.blobPath(name))
		}
	}
}

// Project returns the project with id.
func (d *Data) Project(id string) *Project {
	for i := range d.Projects {
		if d.Projects[i].ID == id {
			return &d.Projects[i]
		}
	}
	return nil
}

// Selected returns the selected env file, or nil.
func (p *Project) Selected() *EnvFile {
	if p == nil || p.SelectedEnvID == "" {
		return nil
	}
	return p.Env(p.SelectedEnvID)
}

// Env returns the env file with id.
func (p *Project) Env(id string) *EnvFile {
	if p == nil {
		return nil
	}
	for i := range p.Envs {
		if p.Envs[i].ID == id {
			return &p.Envs[i]
		}
	}
	return nil
}

// EnvByName returns the env file with an exact name.
func (p *Project) EnvByName(name string) *EnvFile {
	if p == nil {
		return nil
	}
	for i := range p.Envs {
		if p.Envs[i].Name == name {
			return &p.Envs[i]
		}
	}
	return nil
}

// Latest returns the highest version, or nil.
func (e *EnvFile) Latest() *Version {
	if e == nil || len(e.Versions) == 0 {
		return nil
	}
	best := 0
	for i := range e.Versions {
		if e.Versions[i].N > e.Versions[best].N {
			best = i
		}
	}
	return &e.Versions[best]
}

// ByN returns the version numbered n.
func (e *EnvFile) ByN(n int) *Version {
	if e == nil {
		return nil
	}
	for i := range e.Versions {
		if e.Versions[i].N == n {
			return &e.Versions[i]
		}
	}
	return nil
}

// Previous returns the highest version below n.
func (e *EnvFile) Previous(n int) *Version {
	if e == nil {
		return nil
	}
	var best *Version
	for i := range e.Versions {
		if e.Versions[i].N < n && (best == nil || e.Versions[i].N > best.N) {
			best = &e.Versions[i]
		}
	}
	return best
}

func (s *Store) addVersion(env *EnvFile, content []byte) (Version, bool, error) {
	sum := crypto.Hash(content)
	if lat := env.Latest(); lat != nil && lat.SHA256 == sum {
		return *lat, false, nil
	}
	blob, err := s.writeBlob(content)
	if err != nil {
		return Version{}, false, err
	}
	n := 1
	if lat := env.Latest(); lat != nil {
		n = lat.N + 1
	}
	v := Version{
		N:         n,
		CreatedAt: time.Now().UTC(),
		SHA256:    sum,
		Blob:      blob,
		Bytes:     len(content),
	}
	env.Versions = append(env.Versions, v)
	return v, true, nil
}
