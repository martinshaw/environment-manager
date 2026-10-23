package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".envm-*")
	if err != nil {
		return err
	}
	name := f.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(name)
		}
	}()
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := replaceFile(name, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func replaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}

// WriteDisk writes path atomically. An existing file keeps its permission bits.
func WriteDisk(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		if perm := info.Mode().Perm(); perm != 0 {
			mode = perm
		}
	}
	return writeAtomic(path, data, mode)
}

// ReadDisk reads path. exists is false when the file is absent.
func ReadDisk(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

// HashDisk hashes the file at path. A missing file returns MissingHash.
func HashDisk(path string) (string, error) {
	b, ok, err := ReadDisk(path)
	if err != nil {
		return "", err
	}
	if !ok {
		return MissingHash, nil
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// EnvPath is the on-disk .env for a project directory.
func EnvPath(projectPath string) string {
	return filepath.Join(projectPath, ".env")
}
