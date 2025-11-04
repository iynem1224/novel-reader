package library

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"novel_reader/utils"
)

var chapterPattern = regexp.MustCompile(`^第[0-9一二三四五六七八九十百千~-]+章(?:\s*：?\s*.*)?$`)

// LoadLocalNovels scans local library folders and returns Novel structs
func LoadLocalNovels() ([]Novel, error) {
	var novels []Novel

	// Scan all configured library paths
	for _, dir := range utils.AppConfig.Library.Paths {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Only .txt files
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".txt") {
				info, statErr := os.Stat(path)
				if statErr != nil {
					return statErr
				}

				latest, _ := detectLatestChapter(path)

				novels = append(novels, Novel{
					Name:     strings.TrimSuffix(d.Name(), ".txt"),
					Path:     path,
					Latest:   latest,
					Current:  "",
					Modified: info.ModTime(),
					Added:    info.ModTime(),
					IsLocal:  true,
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Load saved progress and merge into Novel structs
	progressMap, _ := utils.Load()
	for i := range novels {
		if p, ok := utils.GetProgress(progressMap, novels[i].Name, "local"); ok {
			if !p.LastRead.IsZero() {
				novels[i].Modified = p.LastRead
			}
			if p.LastChapter != "" {
				novels[i].Current = p.LastChapter
			}
		}
	}

	// Sort by most recently read
	sort.Slice(novels, func(i, j int) bool {
		return novels[i].Modified.After(novels[j].Modified)
	})

	return novels, nil
}

// GroupLocalNovelsByRoot organizes local novels under each configured library path.
func GroupLocalNovelsByRoot(novels []Novel) map[string][]Novel {
	type dirEntry struct {
		raw   string
		clean string
	}

	dirs := make([]dirEntry, len(utils.AppConfig.Library.Paths))
	for i, dir := range utils.AppConfig.Library.Paths {
		dirs[i] = dirEntry{raw: dir, clean: filepath.Clean(dir)}
	}

	grouped := make(map[string][]Novel)
	for _, n := range novels {
		if !n.IsLocal {
			continue
		}
		cleanPath := filepath.Clean(n.Path)
		for _, dir := range dirs {
			if cleanPath == dir.clean || strings.HasPrefix(cleanPath, dir.clean+string(os.PathSeparator)) {
				grouped[dir.raw] = append(grouped[dir.raw], n)
				break
			}
		}
	}

	for dir := range grouped {
		sort.SliceStable(grouped[dir], func(i, j int) bool {
			return grouped[dir][i].Added.After(grouped[dir][j].Added)
		})
	}

	return grouped
}

// ScanLatestChapter scans a local .txt file for the last chapter title
// and updates both the Novel and the saved progress.
func ScanLatestChapter(n *Novel) {
	if !n.IsLocal || n.Path == "" {
		return
	}

	lastChapter, err := detectLatestChapter(n.Path)
	if err != nil || lastChapter == "" {
		return
	}

	n.Latest = lastChapter
	n.Current = lastChapter

	progressMap, _ := utils.Load()
	p, _ := utils.GetProgress(progressMap, n.Name, "local")

	// Ensure Source is always "local"
	p.Source = "local"
	p.LastChapter = lastChapter
	if p.LastRead.IsZero() {
		p.LastRead = time.Now()
	}

	utils.SetProgress(progressMap, n.Name, "local", p)
	utils.Save(progressMap)
}

func detectLatestChapter(path string) (string, error) {
	content, err := readNovelContent(path)
	if err != nil {
		return "", err
	}

	var lastChapter string
	scanner := bufio.NewScanner(strings.NewReader(content))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if chapterPattern.MatchString(line) {
			lastChapter = line
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return lastChapter, nil
}

// LatestChapter reads the local novel file and returns the last detected chapter title.
func LatestChapter(path string) (string, error) {
	return detectLatestChapter(path)
}

func readNovelContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	if decoded, decErr := simplifiedchinese.GB18030.NewDecoder().Bytes(data); decErr == nil {
		return string(decoded), nil
	}
	if decoded, decErr := simplifiedchinese.GBK.NewDecoder().Bytes(data); decErr == nil {
		return string(decoded), nil
	}
	if decoded, decErr := simplifiedchinese.HZGB2312.NewDecoder().Bytes(data); decErr == nil {
		return string(decoded), nil
	}
	return string(data), nil
}
