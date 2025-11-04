package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"novel_reader/lang"
)

const (
	hardVPad = 1
	hardHPad = 2
)

// Reader settings
type ReaderConfig struct {
	VerticalPadding   int `toml:"-"`
	HorizontalPadding int `toml:"-"`
	LineSpacing       int `toml:"line_spacing"`
}

// Library settings
type LibraryConfig struct {
	Paths []string `toml:"paths"`
}

// UI settings
type UIConfig struct {
	Language string `toml:"language"`
}

// Root config
type Config struct {
	Reader  ReaderConfig  `toml:"reader"`
	Library LibraryConfig `toml:"library"`
	UI      UIConfig      `toml:"ui"`
}

// Global variable to hold config
var (
	AppConfig  Config
	configPath string
)

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

// LoadConfig reads config.toml into AppConfig (creates default if missing)
func LoadConfig(path string) {
	configPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// create minimal default template (no paddings)
			AppConfig = Config{
				Reader:  ReaderConfig{LineSpacing: 1},
				Library: LibraryConfig{Paths: []string{}},
				UI:      UIConfig{Language: "en"},
			}
			// ensure dir and write file
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			if err := SaveConfig(); err != nil {
				log.Fatalf("failed to create default config: %v", err)
			}
		} else {
			log.Fatalf("failed to read config: %v", err)
		}
	} else {
		if err := toml.Unmarshal(data, &AppConfig); err != nil {
			log.Fatalf("failed to parse config: %v", err)
		}
	}

	// Hardcode paddings in-memory (not from file)
	AppConfig.Reader.VerticalPadding = hardVPad
	AppConfig.Reader.HorizontalPadding = hardHPad

	// Expand ~ in library paths
	for i, p := range AppConfig.Library.Paths {
		AppConfig.Library.Paths[i] = expandPath(p)
	}

	// Ensure UI language is set and apply locale
	locale := lang.Locale(AppConfig.UI.Language)
	if AppConfig.UI.Language == "" || !lang.SetLocale(locale) {
		locale = lang.LocaleChinese
		_ = lang.SetLocale(locale)
		AppConfig.UI.Language = string(locale)
	} else {
		AppConfig.UI.Language = string(locale)
	}
}

func collapseHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	cleaned := filepath.Clean(path)
	homeClean := filepath.Clean(home)
	if cleaned == homeClean {
		return "~"
	}
	if strings.HasPrefix(cleaned, homeClean+string(os.PathSeparator)) {
		rel := strings.TrimPrefix(cleaned, homeClean+string(os.PathSeparator))
		return filepath.Join("~", rel)
	}
	return path
}

// SaveConfig writes the current AppConfig back to disk (no paddings in file).
func SaveConfig() error {
	if configPath == "" {
		return fmt.Errorf("config path not set")
	}

	cfg := AppConfig

	// Collapse home for file output
	cfg.Library.Paths = make([]string, len(AppConfig.Library.Paths))
	for i, p := range AppConfig.Library.Paths {
		cfg.Library.Paths[i] = collapseHome(p)
	}

	// paddings are tagged toml:"-" so they won't serialize
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, configPath)
}

func Main() {
	homeDir, _ := os.UserHomeDir()
	LoadConfig(homeDir + "/.config/novel_reader/config.toml")
}
