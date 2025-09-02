package utils

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Reader settings
type ReaderConfig struct {
	VerticalPadding   int `toml:"vertical_padding"`
	HorizontalPadding int `toml:"horizontal_padding"`
	LineSpacing       int `toml:"line_spacing"`
}

// Library settings
type LibraryConfig struct {
	Paths []string `toml:"paths"`
}

// Root config
type Config struct {
	Reader  ReaderConfig  `toml:"reader"`
	Library LibraryConfig `toml:"library"`
}

// Global variable to hold config
var AppConfig Config

// expandPath replaces leading "~" with user home dir
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// LoadConfig reads config.toml into AppConfig
func LoadConfig(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	if err := toml.Unmarshal(data, &AppConfig); err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	// Expand ~ in library paths
	for i, p := range AppConfig.Library.Paths {
		AppConfig.Library.Paths[i] = expandPath(p)
	}
}

func Main() {
	homeDir, _ := os.UserHomeDir()
	LoadConfig(homeDir + "/.config/novel_reader/config.toml")
}
