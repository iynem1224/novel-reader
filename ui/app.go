package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"novel_reader/lang"
	"novel_reader/library"
	"novel_reader/utils"
)

type AppState int

const (
	StateLibrary AppState = iota
	StateReader
	StateTOC
)

type AppModel struct {
	state     AppState
	libraryUI LibraryModel
	readerUI  ReaderModel
	tocUI     TOCModel
}

func (m *LibraryModel) ActiveList() *list.Model {
	return &m.lists[m.activeTab]
}

func (m AppModel) handleStateLibrary(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	model, newCmd := m.libraryUI.Update(msg) // update library UI first
	m.libraryUI = model
	cmd = newCmd

	// Handle "Enter" for tab items
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		activeTab := m.libraryUI.activeTab
		selectedItem := m.libraryUI.ActiveList().SelectedItem()
		if selectedItem == nil {
			return m, cmd
		}

		// ---------------- Discovery tab (index 2) ----------------
		if activeTab == 2 {
			if len(m.libraryUI.ActiveList().Items()) == 0 {
				return m, cmd
			}

			if sr, ok := selectedItem.(library.SearchResult); ok {
				if strings.HasPrefix(sr.URL, "/") {
					sr.URL = library.BaseURL + sr.URL
				}
				m.libraryUI.searchLoading = true

				return m, func() tea.Msg {
					novel, err := library.ScrapeAndSaveChapters(sr)
					if err != nil {
						return errMsg{err}
					}
					return novelOpenMsg{Novel: novel}
				}
			}
			return m, cmd
		}

		// ---------------- Local/history tabs ----------------
		if novel, ok := selectedItem.(library.Novel); ok {
			source := "online"
			if novel.IsLocal {
				source = "local"
			}

			var reader ReaderModel
			if novel.IsLocal {
				// Local book: one big .txt file
				reader = NewReaderModel(novel.Path, novel.Name, source)
			} else {
				// Online book: multiple chapter files
				chapters := loadAllChapters(novel.Path)
				reader = NewReaderModelFromFiles(chapters, novel.Name, source)
			}

			m.readerUI = reader
			m.state = StateReader
			openCmd := m.syncWindowSizeCmd()
			if source == "online" {
				cmd = tea.Batch(cmd, openCmd, prefetchAroundCmd(novel.Name, reader.ActualChapterIndex()))
			} else {
				cmd = tea.Batch(cmd, openCmd)
			}
		}
	}

	// ---------------- Handle novelOpenMsg from async scraping ----------------
	if msg, ok := msg.(novelOpenMsg); ok {
		source := "online"
		if msg.Novel.IsLocal {
			source = "local"
		}
		m.libraryUI.scrapeLoading = false
		m.libraryUI.scrapeTitle = ""
		if source == "online" {
			m.libraryUI.clearSearchStatus()
		}

		// Always load all chapters (local or scraped)
		chapters := loadAllChapters(msg.Novel.Path)
		reader := NewReaderModelFromFiles(chapters, msg.Novel.Name, source)

		m.readerUI = reader
		m.state = StateReader
		openCmd := m.syncWindowSizeCmd()
		if source == "online" {
			cmd = tea.Batch(cmd, openCmd, prefetchAroundCmd(msg.Novel.Name, reader.ActualChapterIndex()))
		} else {
			cmd = tea.Batch(cmd, openCmd)
		}
	}

	return m, cmd
}

// Async message to signal library -> App to open a novel
type novelOpenMsg struct {
	Novel library.Novel
}

type chapterChangedMsg struct {
	NovelName string
	Source    string
	Chapter   int
}

type chapterCachedMsg struct {
	NovelName  string
	Chapter    int
	Downloaded bool
}

type chapterReadyMsg struct {
	NovelName string
	Chapter   int
}

func (m AppModel) handleStateReader(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.readerUI, cmd = m.readerUI.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {

		case "esc":
			m.state = StateLibrary

			if m.libraryUI.activeTab == 2 {
				// Discovery tab: just stop loading, keep search results
				m.libraryUI.searchLoading = false
			} else {
				// Reload progress for local/history tabs
				progressMap, _ := utils.Load()
				activeList := m.libraryUI.ActiveList()

				for i, item := range activeList.Items() {
					if v, ok := item.(library.Novel); ok {
						source := "online"
						if v.IsLocal {
							source = "local"
						}
						if p, ok := utils.GetProgress(progressMap, v.Name, source); ok {
							if !p.LastRead.IsZero() {
								v.Modified = p.LastRead
							}
							v.Current = p.LastChapter
							if v.IsLocal {
								if latest, err := library.LatestChapter(v.Path); err == nil && latest != "" {
									v.Latest = latest
								}
							} else {
								if chapters, err := library.LoadChapterList(v.Name); err == nil && len(chapters) > 0 {
									latest := strings.TrimSpace(chapters[len(chapters)-1].Title)
									if latest == "" {
										latest = lang.ChapterTitle(chapters[len(chapters)-1].Index)
									}
									v.Latest = latest
								}
							}
						}
						// Update the item in-place so Bubbletea re-renders it
						activeList.SetItem(i, v)
					}
				}
				m.libraryUI.refreshHistoryList(progressMap)

				// Keep previous selection (optional)
				// activeList.Select(activeList.Index())
			}

			// Force re-render by sending a WindowSizeMsg
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{
					Width:  m.libraryUI.width,
					Height: m.libraryUI.height,
				}
			}

		case "tab", "t": // open TOC
			m.tocUI = NewTOCModel(
				m.readerUI.AllChapters,
				m.readerUI.Width,
				m.readerUI.Height,
				m.readerUI.ActualChapterIndex(),
			)
			m.state = StateTOC
		}
	}

	switch tm := msg.(type) {
	case chapterChangedMsg:
		if tm.Source == "online" && tm.NovelName == m.readerUI.Name {
			cmd = tea.Batch(cmd, prefetchAroundCmd(tm.NovelName, tm.Chapter))
		}
	case chapterCachedMsg:
		if tm.NovelName == m.readerUI.Name && tm.Downloaded && m.readerUI.CacheDir != "" {
			prev := m.readerUI
			chapters := loadAllChapters(prev.CacheDir)
			reader := NewReaderModelFromFiles(chapters, prev.Name, prev.Source)
			reader.Width = prev.Width
			reader.Height = prev.Height
			reader.Style = prev.Style
			target := tm.Chapter
			if target < 0 {
				target = prev.ActualChapterIndex()
			}
			reader.SetCurrentByActual(target)
			reader = reader.WithLoading(false, "")
			m.readerUI = reader
		}
	case chapterReadyMsg:
		if tm.NovelName == m.readerUI.Name && m.readerUI.CacheDir != "" {
			prev := m.readerUI
			chapters := loadAllChapters(prev.CacheDir)
			reader := NewReaderModelFromFiles(chapters, prev.Name, prev.Source)
			reader.Width = prev.Width
			reader.Height = prev.Height
			reader.Style = prev.Style
			reader.SetCurrentByActual(tm.Chapter)
			reader = reader.WithLoading(false, "")
			m.readerUI = reader
			cmd = tea.Batch(cmd, prefetchAroundCmd(prev.Name, tm.Chapter))
		}
	}

	return m, cmd
}

func (m AppModel) handleStateTOC(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.tocUI, cmd = m.tocUI.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.tocUI.list.SetSize(msg.Width-4, msg.Height-2)
	case TOCSelectMsg:
		actual := int(msg)
		m.state = StateReader
		if m.readerUI.Source == "online" {
			title := m.readerUI.TitleForActual(actual)
			loadingText := lang.ReaderLoadingTitle(title)
			m.readerUI = m.readerUI.WithLoading(true, loadingText)
			return m, tea.Batch(cmd, m.syncWindowSizeCmd(), openChapterCmd(m.readerUI.Name, actual))
		}
		m.readerUI.JumpToChapter(actual)
		cmd = tea.Batch(cmd, m.syncWindowSizeCmd())
	case TOCCancelMsg:
		m.state = StateReader
	}

	switch tm := msg.(type) {
	case chapterCachedMsg:
		if tm.NovelName == m.readerUI.Name && tm.Downloaded && m.readerUI.CacheDir != "" {
			prev := m.readerUI
			chapters := loadAllChapters(prev.CacheDir)
			reader := NewReaderModelFromFiles(chapters, prev.Name, prev.Source)
			reader.Width = prev.Width
			reader.Height = prev.Height
			reader.Style = prev.Style
			target := tm.Chapter
			if target < 0 {
				target = prev.ActualChapterIndex()
			}
			reader.SetCurrentByActual(target)
			reader = reader.WithLoading(false, "")
			m.readerUI = reader
		}
	case chapterChangedMsg:
		if tm.Source == "online" && tm.NovelName == m.readerUI.Name {
			cmd = tea.Batch(cmd, prefetchAroundCmd(tm.NovelName, tm.Chapter))
		}
	case chapterReadyMsg:
		if tm.NovelName == m.readerUI.Name && m.readerUI.CacheDir != "" {
			prev := m.readerUI
			chapters := loadAllChapters(prev.CacheDir)
			reader := NewReaderModelFromFiles(chapters, prev.Name, prev.Source)
			reader.Width = prev.Width
			reader.Height = prev.Height
			reader.Style = prev.Style
			reader.SetCurrentByActual(tm.Chapter)
			m.readerUI = reader
			m.state = StateReader
			m.readerUI = m.readerUI.WithLoading(false, "")
			cmd = tea.Batch(cmd, m.syncWindowSizeCmd(), prefetchAroundCmd(prev.Name, tm.Chapter))
		}
	}

	return m, cmd
}

func (m AppModel) syncWindowSizeCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.WindowSizeMsg{
			Width:  m.libraryUI.width,
			Height: m.libraryUI.height,
		}
	}
}

func (m AppModel) Init() tea.Cmd { return nil }

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if _, ok := msg.(languageChangedMsg); ok {
		m.tocUI.ApplyLanguage()
		return m, nil
	}

	switch m.state {
	case StateLibrary:
		return m.handleStateLibrary(msg)
	case StateReader:
		return m.handleStateReader(msg)
	case StateTOC:
		return m.handleStateTOC(msg)
	default:
		return m, nil
	}
}

func (m AppModel) View() string {
	switch m.state {
	case StateLibrary:
		return m.libraryUI.View()
	case StateReader:
		return m.readerUI.View()
	case StateTOC:
		return m.tocUI.View()
	default:
		return lang.Active().Common.UnknownState
	}
}

func NewAppModel() AppModel {
	libraryUI := NewLibraryModel()

	return AppModel{
		state:     StateLibrary,
		libraryUI: libraryUI,
	}
}

func RunApp() {
	app := NewAppModel()

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func openChapterCmd(novelName string, actualIndex int) tea.Cmd {
	return func() tea.Msg {
		_, _, _, err := library.EnsureChapterCached(novelName, actualIndex+1)
		if err != nil {
			return errMsg{err}
		}
		return chapterReadyMsg{NovelName: novelName, Chapter: actualIndex}
	}
}

func prefetchAroundCmd(novelName string, zeroIndex int) tea.Cmd {
	return func() tea.Msg {
		chapters, err := library.LoadChapterList(novelName)
		if err != nil {
			// Ensure the current chapter is cached which will also create the chapter list
			if _, _, _, ensureErr := library.EnsureChapterCached(novelName, zeroIndex+1); ensureErr != nil {
				return errMsg{ensureErr}
			}
			chapters, err = library.LoadChapterList(novelName)
			if err != nil {
				return errMsg{err}
			}
		}

		total := len(chapters)
		if total == 0 {
			return chapterCachedMsg{NovelName: novelName, Chapter: zeroIndex}
		}

		current := zeroIndex + 1
		targets := []int{current}
		if current > 1 {
			targets = append(targets, current-1)
		}
		if current < total {
			targets = append(targets, current+1)
		}

		downloaded := false
		for _, idx := range targets {
			_, _, didDownload, err := library.EnsureChapterCached(novelName, idx)
			if err != nil {
				return errMsg{err}
			}
			if didDownload {
				downloaded = true
			}
		}

		return chapterCachedMsg{
			NovelName:  novelName,
			Chapter:    zeroIndex,
			Downloaded: downloaded,
		}
	}
}

func loadAllChapters(novelDir string) []string {
	files, _ := os.ReadDir(novelDir)
	var chapters []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".txt") {
			chapters = append(chapters, filepath.Join(novelDir, f.Name()))
		}
	}

	// Numeric sort: 1.txt < 2.txt < 10.txt
	sort.Slice(chapters, func(i, j int) bool {
		iname := strings.TrimSuffix(filepath.Base(chapters[i]), ".txt")
		jname := strings.TrimSuffix(filepath.Base(chapters[j]), ".txt")

		inum, err1 := strconv.Atoi(iname)
		jnum, err2 := strconv.Atoi(jname)

		if err1 == nil && err2 == nil {
			return inum < jnum
		}
		// fallback: string compare
		return chapters[i] < chapters[j]
	})

	return chapters
}
