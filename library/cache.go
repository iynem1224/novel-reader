package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"novel_reader/lang"
	"novel_reader/utils"
)

type CachedNovel struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	URL           string `json:"url"`
	LastScraped   string `json:"last_scraped"`
	TotalChapters int    `json:"total_chapters"`
}

func CacheDir() string {
	return filepath.Join(os.Getenv("HOME"), ".config/novel_reader/cache")
}

func NovelCachePath(title string) string {
	return filepath.Join(CacheDir(), title)
}

func SaveChapter(title string, chapterNum int, content string) error {
	dir := NovelCachePath(title)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, fmt.Sprintf("%d.txt", chapterNum))
	return os.WriteFile(path, []byte(content), 0644)
}

func LoadChapter(title string, chapterNum int) (string, error) {
	path := filepath.Join(NovelCachePath(title), fmt.Sprintf("%d.txt", chapterNum))
	data, err := os.ReadFile(path)
	return string(data), err
}

func SaveMeta(title string, meta CachedNovel) error {
	dir := NovelCachePath(title)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "meta.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(meta)
}

func LoadMeta(title string) (CachedNovel, error) {
	var meta CachedNovel
	path := filepath.Join(NovelCachePath(title), "meta.json")
	f, err := os.Open(path)
	if err != nil {
		return meta, err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&meta)
	return meta, err
}

func chapterListPath(title string) string {
	return filepath.Join(NovelCachePath(title), "chapters.json")
}

func SaveChapterList(title string, chapters []ChapterLink) error {
	if len(chapters) == 0 {
		return fmt.Errorf("no chapters to save")
	}

	dir := NovelCachePath(title)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := chapterListPath(title)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(chapters)
}

func LoadChapterList(title string) ([]ChapterLink, error) {
	path := chapterListPath(title)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var chapters []ChapterLink
	if err := json.NewDecoder(f).Decode(&chapters); err != nil {
		return nil, err
	}
	return chapters, nil
}

// LoadAllCachedNovels returns all cached online novels as library.Novel structs
func LoadAllCachedNovels() ([]Novel, error) {
	cacheDir := CacheDir()
	dirs, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, err
	}

	progressMap, _ := utils.Load()
	var novels []Novel

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		dirPath := NovelCachePath(d.Name())
		info, err := os.Stat(dirPath)
		var dirMod time.Time
		if err == nil {
			dirMod = info.ModTime()
		}

		meta, err := LoadMeta(d.Name())
		if err != nil {
			continue
		}

		addedTime := dirMod
		if meta.LastScraped != "" {
			if t, err := time.Parse(time.RFC3339, meta.LastScraped); err == nil {
				addedTime = t
			}
		}

		novel := Novel{
			Name:      d.Name(),
			Path:      dirPath,
			Latest:    "",
			Current:   "",
			Modified:  time.Now(), // will overwrite from progress if available
			Added:     addedTime,
			OnlineURL: meta.URL,
			IsLocal:   false,
		}
		novel.Author = strings.TrimSpace(meta.Author)

		if chapters, err := LoadChapterList(d.Name()); err == nil && len(chapters) > 0 {
			last := chapters[len(chapters)-1]
			latestTitle := strings.TrimSpace(last.Title)
			if latestTitle == "" {
				latestTitle = lang.ChapterTitle(last.Index)
			}
			novel.Latest = latestTitle
		}

		if p, ok := utils.GetProgress(progressMap, novel.Name, "online"); ok {
			if !p.LastRead.IsZero() {
				novel.Modified = p.LastRead
			}
			if p.LastChapter != "" {
				novel.Current = p.LastChapter
			}
		} else if meta.LastScraped != "" {
			if t, err := time.Parse(time.RFC3339, meta.LastScraped); err == nil {
				novel.Modified = t
			}
		}

		if novel.Latest == "" {
			novel.Latest = lang.ChapterTitle(meta.TotalChapters)
		}

		novels = append(novels, novel)
	}

	return novels, nil
}
