package utils

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Progress struct {
	Chapter     int       `json:"chapter"`
	Page        int       `json:"page"`
	LastRead    time.Time `json:"last_read"`
	LastChapter string    `json:"last_chapter"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "novel_reader"), nil
}

func progressFile() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "progress.json"), nil
}

func Load() (map[string]Progress, error) {
	path, err := progressFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]Progress), nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]Progress
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func Save(m map[string]Progress) error {
	path, err := progressFile()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
