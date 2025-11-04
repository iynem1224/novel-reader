package library

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/bubbles/list"
)

const BaseURL = "https://www.22biqu.com"
const searchURL = BaseURL + "/ss/"

// ------------------ SearchResult type ------------------
type SearchResult struct {
	Name       string
	Author     string
	Category   string
	URL        string
	Latest     string
	LatestURL  string
	UpdateTime string
}

// Implement list.Item
func (sr SearchResult) Title() string       { return sr.Name }
func (sr SearchResult) Description() string { return fmt.Sprintf("%s | %s", sr.Author, sr.Latest) }
func (sr SearchResult) FilterValue() string { return sr.Name + " " + sr.Author }

// Convert SearchResult to Novel
func (sr SearchResult) ToNovel() Novel {
	path := NovelCachePath(sr.Name)
	return Novel{
		Name:      sr.Name,
		Author:    sr.Author,
		Path:      path,
		Latest:    sr.Latest,
		Modified:  time.Now(),
		Added:     time.Now(),
		OnlineURL: sr.URL,
		IsLocal:   false,
	}
}

// ------------------ Search function ------------------
func SearchNovel(query string) ([]list.Item, error) {
	form := url.Values{}
	form.Set("searchkey", query)

	req, err := http.NewRequest("POST", searchURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []list.Item
	doc.Find(".txt-list li").Each(func(i int, sel *goquery.Selection) {
		// Skip header row
		if sel.Find("b").Length() > 0 {
			return
		}

		category := strings.TrimSpace(sel.Find(".s1").Text())
		titleSel := sel.Find(".s2 a")
		title := strings.TrimSpace(titleSel.Text())
		href, _ := titleSel.Attr("href")
		if !strings.HasPrefix(href, "http") {
			href = BaseURL + href
		}
		chapterSel := sel.Find(".s3 a")
		latest := strings.TrimSpace(chapterSel.Text())
		author := strings.TrimSpace(sel.Find(".s4").Text())
		updateTime := strings.TrimSpace(sel.Find(".s5").Text())

		sr := SearchResult{
			Category:   category,
			Name:       title,
			URL:        href,
			Latest:     latest,
			LatestURL:  chapterSel.Text(),
			Author:     author,
			UpdateTime: updateTime,
		}

		// Keep SearchResult in the list, not Novel
		items = append(items, sr)
	})

	return items, nil
}
