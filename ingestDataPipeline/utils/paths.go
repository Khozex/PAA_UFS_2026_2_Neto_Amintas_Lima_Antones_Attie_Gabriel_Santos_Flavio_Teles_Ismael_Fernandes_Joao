package utils

import (
	"os"
	"path/filepath"
)

func Resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(moduleRoot(), path)
}

func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
