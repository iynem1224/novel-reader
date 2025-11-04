package utils

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Progress tracks reading progress of a novel
type Progress struct {
	Source      string    `json:"source"`            // "local" or "online"
	Page        int       `json:"page"`              // current page (for both local and online)
	LastRead    time.Time `json:"last_read"`         // timestamp of last read
	LastChapter string    `json:"last_chapter"`      // latest chapter title
	Chapter     int       `json:"chapter,omitempty"` // only used for online novels
}

// ---------------- Paths ----------------
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

// ---------------- Helper for compound keys ----------------
func makeKey(name, source string) string {
	return name + "|" + source
}

func parseKey(key string) (name, source string) {
	parts := strings.SplitN(key, "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, "local"
}

// ---------------- Load progress ----------------
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

	var raw map[string]Progress
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	progressMap := make(map[string]Progress)
	for k, v := range raw {
		name, source := parseKey(k)
		if v.Source == "" {
			v.Source = source
		}
		if name != "" && v.Source != "" {
			progressMap[makeKey(name, v.Source)] = v
		}
	}

	return progressMap, nil
}

// ---------------- Save progress ----------------
func Save(m map[string]Progress) error {
	path, err := progressFile()
	if err != nil {
		return err
	}

	saveMap := make(map[string]Progress)
	for key, v := range m {
		name, source := parseKey(key)
		if name == "" || source == "" {
			continue // skip invalid entries
		}
		saveMap[makeKey(name, source)] = v
	}

	data, err := json.MarshalIndent(saveMap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ---------------- Convenience ----------------

// GetProgress safely retrieves a progress entry by name + source
func GetProgress(m map[string]Progress, name, source string) (Progress, bool) {
	if name == "" || source == "" {
		return Progress{}, false
	}
	p, ok := m[makeKey(name, source)]
	return p, ok
}

// SetProgress safely updates a progress entry
func SetProgress(m map[string]Progress, name, source string, p Progress) {
	if name == "" || source == "" {
		return
	}
	p.Source = source
	m[makeKey(name, source)] = p
}

// DeleteProgress removes a progress entry for the given name and source.
func DeleteProgress(name, source string) error {
	if name == "" || source == "" {
		return nil
	}

	progressMap, err := Load()
	if err != nil {
		return err
	}

	key := makeKey(name, source)
	if _, ok := progressMap[key]; !ok {
		return nil
	}
	delete(progressMap, key)

	return Save(progressMap)
}
