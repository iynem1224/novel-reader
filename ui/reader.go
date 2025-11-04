package ui

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"novel_reader/lang"
	"novel_reader/library"
	"novel_reader/utils"
)

var chapterPattern = regexp.MustCompile(`^第[0-9一二三四五六七八九十百千~-]+章(?:\s*：?\s*.*)?$`)

type Chapter struct {
	Title string
	Line  int // Line number in Content where chapter starts
}

type TOCChapter struct {
	Title string
	Index int
}

type ReaderModel struct {
	Name           string
	CacheDir       string
	Content        []string
	TOC            []Chapter
	TotalChapters  int
	AllChapters    []TOCChapter
	LoadedIndices  []int
	Loading        bool
	LoadingText    string
	currentChapter int
	Width          int
	Height         int
	Page           int
	Style          gloss.Style
	Source         string // <-- add this
}

func (m ReaderModel) Init() tea.Cmd { return nil }

func (m ReaderModel) Update(msg tea.Msg) (ReaderModel, tea.Cmd) {
	prevChapter := m.currentChapter
	prevActual := m.actualIndexForLoaded(prevChapter)
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "right", "l", "j":
			_, lastIndex, _ := m.displayedContent()
			chapterEnd := len(m.Content) - 1
			if m.currentChapter < len(m.TOC)-1 {
				chapterEnd = m.TOC[m.currentChapter+1].Line - 1
			}
			if lastIndex < chapterEnd {
				m.Page++
			} else if m.currentChapter < len(m.TOC)-1 {
				m.currentChapter++
				m.Page = 0
			}

		case "left", "h", "k":
			if m.Page > 0 {
				m.Page--
			} else if m.currentChapter > 0 {
				m.currentChapter--
				m.Page = m.totalPagesInChapter(m.currentChapter) - 1
			}

		case "ctrl+d": // jump to next chapter
			if m.currentChapter < len(m.TOC)-1 {
				m.currentChapter++
				m.Page = 0
			}

		case "ctrl+u": // jump to previous chapter
			if m.currentChapter > 0 {
				m.currentChapter--
				m.Page = 0
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Style = ReaderStyle(m.Width)
	}

	// Save progress using correct source
	progressMap, _ := utils.Load()
	lastChapter := ""
	if len(m.TOC) > 0 && m.currentChapter < len(m.TOC) {
		lastChapter = m.TOC[m.currentChapter].Title
	}

	currentActual := m.ActualChapterIndex()

	utils.SetProgress(progressMap, m.Name, m.Source, utils.Progress{
		Chapter:     currentActual,
		Page:        m.Page,
		LastRead:    time.Now(),
		LastChapter: lastChapter,
		Source:      m.Source,
	})
	utils.Save(progressMap)

	if currentActual != prevActual && m.Source == "online" {
		chapter := currentActual
		cmds = append(cmds, func() tea.Msg {
			return chapterChangedMsg{
				NovelName: m.Name,
				Source:    m.Source,
				Chapter:   chapter,
			}
		})
	}

	return m, tea.Batch(cmds...)
}

func (m ReaderModel) View() string {
	if m.Loading {
		text := m.LoadingText
		if strings.TrimSpace(text) == "" {
			text = lang.Active().Reader.LoadingDefault
		}
		return ReaderLoadingStyle.Width(m.Width).Render(text)
	}

	lines, _, _ := m.displayedContent()
	visible := strings.Join(lines, strings.Repeat("\n", utils.AppConfig.Reader.LineSpacing+1))
	return m.Style.Render(visible)
}

func parseTOC(lines []string) []Chapter {
	var toc []Chapter
	for i, line := range lines {
		// English-style chapters
		if strings.HasPrefix(line, "Chapter ") {
			toc = append(toc, Chapter{Title: line, Line: i})
			continue
		}

		// Chinese-style chapters like "第12章" or "第一百二十三章"
		if chapterPattern.MatchString(line) {
			toc = append(toc, Chapter{Title: line, Line: i})
		}
	}
	return toc
}

// Return how many terminal rows a line takes when wrapped.
func (m ReaderModel) countWrapped(line string) int {
	usableWidth := m.Width - (2 * utils.AppConfig.Reader.HorizontalPadding)
	if runewidth.StringWidth(line) == 0 {
		// Empty line should still take one row
		return 1
	}
	lineWidth := runewidth.StringWidth(line)
	return (lineWidth + usableWidth - 1) / usableWidth
}

func (m ReaderModel) displayedContentFrom(start, end int) ([]string, int, int) {
	usableHeight := m.Height - (utils.AppConfig.Reader.VerticalPadding * 2)
	var result []string
	lastIndex := start
	avail := usableHeight

	for i := start; i <= end; i++ {
		rows := m.countWrapped(m.Content[i])
		if avail-rows < 0 {
			break
		}
		avail -= rows
		if i < end {
			avail -= utils.AppConfig.Reader.LineSpacing
		}
		result = append(result, m.Content[i])
		lastIndex = i
	}

	return result, lastIndex, avail
}

// Calculate which lines fit on the current page.
func (m ReaderModel) displayedContent() ([]string, int, int) {
	if len(m.TOC) == 0 {
		return m.Content, 0, 0
	}
	chapter := m.TOC[m.currentChapter]
	start := chapter.Line
	end := len(m.Content) - 1
	if m.currentChapter < len(m.TOC)-1 {
		end = m.TOC[m.currentChapter+1].Line - 1
	}

	usableHeight := m.Height - (utils.AppConfig.Reader.VerticalPadding * 2)
	pageStart := start

	// Skip pages before m.Page
	for p := 0; p < m.Page; p++ {
		avail := usableHeight
		for i := pageStart; i <= end; i++ {
			rows := m.countWrapped(m.Content[i])
			if avail-rows < 0 {
				break
			}
			avail -= rows
			if i < end {
				avail -= utils.AppConfig.Reader.LineSpacing
			}
			pageStart = i + 1
		}
	}

	// Current page
	return m.displayedContentFrom(pageStart, end)
}

func NewReaderModel(filePath, name string, source string) ReaderModel {
	lines := utils.ExtractContent(filePath)
	toc := parseTOC(lines)
	if len(toc) == 0 {
		toc = []Chapter{{Title: "", Line: 0}}
	}

	loadedIndices := make([]int, len(toc))
	allChapters := make([]TOCChapter, len(toc))
	for i, ch := range toc {
		loadedIndices[i] = i
		allChapters[i] = TOCChapter{Title: ch.Title, Index: i}
	}

	// Default values
	currentChapter := 0
	page := 0
	cacheDir := filepath.Dir(filePath)

	// Load saved progress using correct source
	progressMap, _ := utils.Load()
	if p, ok := utils.GetProgress(progressMap, name, source); ok {
		currentChapter = p.Chapter
		page = p.Page
		if currentChapter < 0 || currentChapter >= len(toc) {
			currentChapter = 0
		}
		if page < 0 {
			page = 0
		}
	}

	return ReaderModel{
		Content:        lines,
		TOC:            toc,
		CacheDir:       cacheDir,
		TotalChapters:  len(allChapters),
		AllChapters:    allChapters,
		LoadedIndices:  loadedIndices,
		currentChapter: currentChapter,
		Page:           page,
		Name:           name,
		Source:         source, // <-- set source here
	}
}

func (m *ReaderModel) JumpToChapter(index int) {
	if index >= 0 && index < len(m.TOC) {
		m.currentChapter = index
		m.Page = 0
		m.Loading = false
		m.LoadingText = ""
	}
}

func (m ReaderModel) totalPagesInChapter(chapIndex int) int {
	chapter := m.TOC[chapIndex]
	start := chapter.Line
	end := len(m.Content) - 1
	if chapIndex < len(m.TOC)-1 {
		end = m.TOC[chapIndex+1].Line - 1
	}

	usableHeight := m.Height - (utils.AppConfig.Reader.VerticalPadding * 2)
	pages := 0
	pageStart := start

	for pageStart <= end {
		avail := usableHeight
		for i := pageStart; i <= end; i++ {
			rows := m.countWrapped(m.Content[i])
			if avail-rows < 0 {
				break
			}
			avail -= rows
			if i < end {
				avail -= utils.AppConfig.Reader.LineSpacing
			}
			pageStart = i + 1
		}
		pages++
	}
	return pages
}

func NewReaderModelFromFiles(files []string, name string, source string) ReaderModel {
	cacheDir := ""
	if len(files) > 0 {
		cacheDir = filepath.Dir(files[0])
	}

	var allLines []string
	var toc []Chapter
	var loadedIndices []int
	var allChapters []TOCChapter

	if source == "online" {
		chapterList, _ := library.LoadChapterList(name)
		indexToTitle := make(map[int]string, len(chapterList))
		for _, ch := range chapterList {
			indexToTitle[ch.Index] = strings.TrimSpace(ch.Title)
			title := strings.TrimSpace(ch.Title)
			if title == "" {
				title = lang.ChapterTitle(ch.Index)
			}
			allChapters = append(allChapters, TOCChapter{
				Title: title,
				Index: ch.Index - 1,
			})
		}

		for _, f := range files {
			lines := utils.ExtractContent(f)
			start := len(allLines)
			allLines = append(allLines, lines...)

			title := ""
			actualIndex := len(loadedIndices)
			base := filepath.Base(f)
			if num, err := strconv.Atoi(strings.TrimSuffix(base, ".txt")); err == nil {
				actualIndex = num - 1
				if t, ok := indexToTitle[num]; ok && t != "" {
					title = t
				}
			}
			if title == "" {
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" {
						title = trimmed
						break
					}
				}
			}
			if title == "" {
				title = lang.ChapterTitle(len(toc) + 1)
			}

			loadedIndices = append(loadedIndices, actualIndex)
			toc = append(toc, Chapter{Title: title, Line: start})
		}

		if len(toc) == 0 {
			toc = []Chapter{{Title: "", Line: 0}}
			if len(loadedIndices) == 0 {
				loadedIndices = append(loadedIndices, 0)
			}
		}

		if len(allChapters) == 0 {
			for i := range toc {
				allChapters = append(allChapters, TOCChapter{Title: toc[i].Title, Index: i})
			}
		}
	} else {
		for _, f := range files {
			lines := utils.ExtractContent(f)
			allLines = append(allLines, lines...)
		}

		toc = parseTOC(allLines)
		if len(toc) == 0 {
			toc = []Chapter{{Title: "", Line: 0}}
		}
		for i := range toc {
			loadedIndices = append(loadedIndices, i)
			allChapters = append(allChapters, TOCChapter{Title: toc[i].Title, Index: i})
		}
	}

	if len(loadedIndices) == 0 {
		for i := range toc {
			loadedIndices = append(loadedIndices, i)
		}
	}
	if len(allChapters) == 0 {
		for i := range toc {
			allChapters = append(allChapters, TOCChapter{Title: toc[i].Title, Index: i})
		}
	}

	currentChapter := 0
	page := 0

	progressMap, _ := utils.Load()
	if p, ok := utils.GetProgress(progressMap, name, source); ok {
		currentChapter = p.Chapter
		page = p.Page
		if currentChapter < 0 {
			currentChapter = 0
		}
		// Progress stores actual chapter index (zero-based). Map to loaded index.
		if len(loadedIndices) > 0 {
			found := false
			for idx, actual := range loadedIndices {
				if actual == currentChapter {
					currentChapter = idx
					found = true
					break
				}
			}
			if !found {
				currentChapter = 0
			}
		} else if currentChapter >= len(toc) {
			currentChapter = len(toc) - 1
		}
		if page < 0 {
			page = 0
		}
	}

	return ReaderModel{
		Content:        allLines,
		TOC:            toc,
		CacheDir:       cacheDir,
		TotalChapters:  len(allChapters),
		AllChapters:    allChapters,
		LoadedIndices:  loadedIndices,
		currentChapter: currentChapter,
		Page:           page,
		Name:           name,
		Source:         source,
	}
}

func (m ReaderModel) actualIndexForLoaded(idx int) int {
	if idx >= 0 && idx < len(m.LoadedIndices) {
		return m.LoadedIndices[idx]
	}
	return idx
}

func (m ReaderModel) ActualChapterIndex() int {
	return m.actualIndexForLoaded(m.currentChapter)
}

func (m ReaderModel) indexForActual(actual int) int {
	for idx, v := range m.LoadedIndices {
		if v == actual {
			return idx
		}
	}
	return -1
}

func (m *ReaderModel) SetCurrentByActual(actual int) {
	if idx := m.indexForActual(actual); idx >= 0 {
		m.currentChapter = idx
		m.Page = 0
		m.Loading = false
		m.LoadingText = ""
	}
}

func (m ReaderModel) TitleForActual(actual int) string {
	for _, ch := range m.AllChapters {
		if ch.Index == actual && strings.TrimSpace(ch.Title) != "" {
			return ch.Title
		}
	}
	return lang.ChapterTitle(actual + 1)
}

func (m ReaderModel) WithLoading(loading bool, text string) ReaderModel {
	m.Loading = loading
	m.LoadingText = text
	return m
}
