package ui

import (
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"novel_reader/utils"
)

var chapterPattern = regexp.MustCompile(`^第[0-9一二三四五六七八九十百千~-]+章(?:\s.*)?$`)

type Chapter struct {
	Title string
	Line  int // Line number in Content where chapter starts
}

type ReaderModel struct {
	Name           string
	Content        []string
	TOC            []Chapter
	currentChapter int

	Width  int
	Height int
	Page   int
	Style  gloss.Style
}

func (m ReaderModel) Init() tea.Cmd { return nil }

func (m ReaderModel) Update(msg tea.Msg) (ReaderModel, tea.Cmd) {
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

	// Save progress
	progressMap, _ := utils.Load()
	lastChapter := ""
	if len(m.TOC) > 0 {
    	lastChapter = m.TOC[len(m.TOC)-1].Title
	}

	progressMap[m.Name] = utils.Progress{
    	Chapter:     m.currentChapter,
    	Page:        m.Page,
    	LastRead:    time.Now(),
    	LastChapter: lastChapter,
	}
	utils.Save(progressMap)

	return m, nil
}

func (m ReaderModel) View() string {
	lines, _, _ := m.displayedContent()
	visible := strings.Join(lines, strings.Repeat("\n", utils.AppConfig.Reader.LineSpacing+1))
	return m.Style.Render(visible)
}

func parseTOC(lines []string) []Chapter {
	var toc []Chapter
	for i, line := range lines {
		line = strings.TrimSpace(line)

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

func NewReaderModel(filePath, name string) ReaderModel {
	lines := utils.ExtractContent(filePath)
	toc := parseTOC(lines)
	return ReaderModel{
		Content:        lines,
		TOC:            toc,
		currentChapter: 0,
		Page:           0,
		Name:           name, // store the name for progress saving
	}
}

func (m *ReaderModel) JumpToChapter(index int) {
	if index >= 0 && index < len(m.TOC) {
		m.currentChapter = index
		m.Page = 0
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
