package library

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"novel_reader/utils"

	"github.com/PuerkitoBio/goquery"
)

var httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

// ----------------------------
// TYPES & MODELS
// ----------------------------
type ChapterLink struct {
	Index int    `json:"index"`
	Link  string `json:"link"`
	Title string `json:"title"`
}

// ----------------------------
// UTILS
// ----------------------------
func fetchHTML(url string) (*goquery.Document, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return goquery.NewDocumentFromReader(resp.Body)
}

func cleanChapterTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "第") && strings.Contains(raw, "章") {
		idx := strings.Index(raw, "第")
		return strings.TrimSpace(raw[idx:])
	}
	return raw
}

// ----------------------------
// GET CHAPTER LINKS
// ----------------------------
func GetChapterLinks(novelURL, latestChapter string) ([]ChapterLink, error) {
	var chapters []ChapterLink
	for page := 1; page <= 500; page++ {
		url := fmt.Sprintf("%s%d", novelURL, page)
		doc, err := fetchHTML(url)
		if err != nil {
			return nil, err
		}

		ul := doc.Find("ul.section-list.fix").Eq(1)
		if ul.Length() == 0 {
			break
		}

		var last string
		ul.Find("li").Each(func(i int, s *goquery.Selection) {
			link, _ := s.Find("a").Attr("href")
			link = BaseURL + link
			rawTitle := strings.TrimSpace(s.Text())
			title := cleanChapterTitle(rawTitle)
			chapters = append(chapters, ChapterLink{
				Index: len(chapters) + 1,
				Link:  link,
				Title: title,
			})
			last = rawTitle
		})

		if last == latestChapter {
			break
		}
	}
	return chapters, nil
}

// ----------------------------
// SCRAPE SINGLE CHAPTER
// ----------------------------
func ScrapeChapter(ch ChapterLink) (string, error) {
	var content strings.Builder
	for subpage := 1; ; subpage++ {
		pageURL := ch.Link
		if subpage > 1 {
			pageURL = strings.Replace(ch.Link, ".html", fmt.Sprintf("_%d.html", subpage), 1)
		}

		doc, err := fetchHTML(pageURL)
		if err != nil {
			if subpage == 1 {
				return "", err
			}
			break
		}

		pContent := ""
		doc.Find("#content p").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				pContent += "　　" + text + "\n"
			}
		})

		if pContent == "" {
			text := strings.TrimSpace(doc.Find("#content").Text())
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					pContent += "　　" + line + "\n"
				}
			}
		}

		title := strings.TrimSpace(doc.Find("h1.title").Text())
		if subpage == 1 {
			content.WriteString(fmt.Sprintf("%s\n%s\n", title, pContent))
		} else {
			content.WriteString(pContent)
		}

		if pContent == "" {
			break
		}
	}
	return content.String(), nil
}

// ----------------------------
// SCRAPE & SAVE ALL CHAPTERS
// ----------------------------
func ScrapeAndSaveChapters(sr SearchResult) (Novel, error) {
	if strings.HasPrefix(sr.URL, "/") {
		sr.URL = BaseURL + sr.URL
	}

	author := strings.TrimSpace(sr.Author)
	latest := strings.TrimSpace(sr.Latest)
	if author == "" || latest == "" {
		if doc, err := fetchHTML(sr.URL); err == nil {
			doc.Find(".top .fix p").Each(func(_ int, sel *goquery.Selection) {
				text := strings.TrimSpace(sel.Text())
				switch {
				case author == "" && (strings.HasPrefix(text, "作") || strings.Contains(text, "作者")):
					if parts := strings.Split(text, "："); len(parts) == 2 {
						author = strings.TrimSpace(parts[1])
					} else if parts := strings.Split(text, ":"); len(parts) == 2 {
						author = strings.TrimSpace(parts[1])
					}
				case latest == "":
					if sel.HasClass("xs-show") {
						return
					}
					if strings.Contains(text, "最新章节") {
						if t := strings.TrimSpace(sel.Find("a").Text()); t != "" {
							latest = t
						} else if parts := strings.Split(text, "："); len(parts) == 2 {
							latest = strings.TrimSpace(parts[1])
						} else if parts := strings.Split(text, ":"); len(parts) == 2 {
							latest = strings.TrimSpace(parts[1])
						}
					}
				}
			})
		}
	}
	if author != "" {
		sr.Author = author
	}
	if latest != "" {
		sr.Latest = latest
	}

	cacheDir := NovelCachePath(sr.Name)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return Novel{}, err
	}

	// Load progress map
	progressMap, _ := utils.Load()
	prog := utils.Progress{}
	if p, ok := utils.GetProgress(progressMap, sr.Name, "online"); ok {
		prog = p
	}

	chapters, err := loadOrRefreshChapterList(sr.Name, sr.URL, sr.Latest)
	if err != nil || len(chapters) == 0 {
		return Novel{}, fmt.Errorf("no chapters found: %w", err)
	}

	// Determine the chapter we should show (1-based index)
	currentIndex := prog.Chapter + 1
	if currentIndex <= 0 || currentIndex > len(chapters) {
		currentIndex = 1
	}
	currentChapter := chapters[currentIndex-1]
	latestOverall := chapters[len(chapters)-1]

	chapterPath := filepath.Join(cacheDir, fmt.Sprintf("%d.txt", currentChapter.Index))
	if _, err := os.Stat(chapterPath); errors.Is(err, os.ErrNotExist) {
		content, err := ScrapeChapterWithSubpages(currentChapter)
		if err != nil {
			return Novel{}, fmt.Errorf("failed to scrape chapter %d: %w", currentChapter.Index, err)
		}
		if err := os.WriteFile(chapterPath, []byte(content), 0644); err != nil {
			return Novel{}, fmt.Errorf("failed to save chapter %d: %w", currentChapter.Index, err)
		}
	}

	novel := Novel{
		Name:      sr.Name,
		Author:    strings.TrimSpace(sr.Author),
		Path:      cacheDir,
		Latest:    strings.TrimSpace(latestOverall.Title),
		Current:   currentChapter.Title,
		Modified:  time.Now(),
		Added:     time.Now(),
		OnlineURL: sr.URL,
		IsLocal:   false,
	}

	// Update progress map
	prog.LastChapter = currentChapter.Title
	prog.LastRead = time.Now()
	prog.Chapter = currentIndex - 1
	prog.Source = "online"
	utils.SetProgress(progressMap, sr.Name, "online", prog)
	utils.Save(progressMap)

	SaveMeta(sr.Name, CachedNovel{
		Title:         sr.Name,
		Author:        sr.Author,
		URL:           sr.URL,
		LastScraped:   time.Now().Format(time.RFC3339),
		TotalChapters: len(chapters),
	})

	return novel, nil
}

// ----------------------------
// SCRAPE SINGLE CHAPTER INCLUDING SUBPAGES
// ----------------------------
func ScrapeChapterWithSubpages(ch ChapterLink) (string, error) {
	var content strings.Builder
	previousContent := ""

	for subpage := 1; ; subpage++ {
		pageURL := ch.Link
		if subpage > 1 {
			pageURL = strings.Replace(ch.Link, ".html", fmt.Sprintf("_%d.html", subpage), 1)
		}

		doc, err := fetchHTML(pageURL)
		if err != nil {
			if subpage == 1 {
				return "", err
			}
			break
		}

		var pContent strings.Builder
		doc.Find("#content p").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				pContent.WriteString("　　" + text + "\n")
			}
		})

		if pContent.Len() == 0 {
			text := strings.TrimSpace(doc.Find("#content").Text())
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					pContent.WriteString("　　" + line + "\n")
				}
			}
		}

		currentContent := pContent.String()
		if currentContent == previousContent {
			break
		}
		previousContent = currentContent

		if subpage == 1 {
			title := strings.TrimSpace(doc.Find("h1.title").Text())
			content.WriteString(fmt.Sprintf("%s\n%s", title, currentContent))
		} else {
			content.WriteString(currentContent)
		}
	}

	return content.String(), nil
}

func loadOrRefreshChapterList(title, novelURL, latest string) ([]ChapterLink, error) {
	chapters, err := LoadChapterList(title)
	refresh := err != nil || len(chapters) == 0

	if !refresh && latest != "" {
		found := false
		for _, ch := range chapters {
			if strings.TrimSpace(ch.Title) == strings.TrimSpace(latest) {
				found = true
				break
			}
		}
		refresh = !found
	}

	if refresh {
		chapters, err = GetChapterLinks(novelURL, latest)
		if err != nil {
			return nil, err
		}
		if len(chapters) == 0 {
			return nil, fmt.Errorf("no chapters found")
		}
		if err := SaveChapterList(title, chapters); err != nil {
			return nil, err
		}
	}

	return chapters, nil
}

func EnsureChapterCached(title string, index int) (ChapterLink, string, bool, error) {
	meta, err := LoadMeta(title)
	if err != nil {
		return ChapterLink{}, "", false, err
	}

	if meta.URL == "" {
		return ChapterLink{}, "", false, fmt.Errorf("missing novel URL for %s", title)
	}

	chapters, err := loadOrRefreshChapterList(title, meta.URL, "")
	if err != nil {
		return ChapterLink{}, "", false, err
	}

	if index <= 0 || index > len(chapters) {
		return ChapterLink{}, "", false, fmt.Errorf("chapter index %d out of range", index)
	}

	ch := chapters[index-1]
	cacheDir := NovelCachePath(title)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return ChapterLink{}, "", false, err
	}

	path := filepath.Join(cacheDir, fmt.Sprintf("%d.txt", ch.Index))
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		content, err := ScrapeChapterWithSubpages(ch)
		if err != nil {
			return ChapterLink{}, "", false, err
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return ChapterLink{}, "", false, err
		}
		return ch, path, true, nil
	}

	return ch, path, false, nil
}
