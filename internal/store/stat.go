package store

import "os"

func statPath(path string) (*statInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &statInfo{isDir: info.IsDir()}, nil
}
