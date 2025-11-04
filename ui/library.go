package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/wordwrap"

	"novel_reader/lang"
	"novel_reader/library"
	"novel_reader/utils"
)

// ---------------- LibraryModel ----------------
type LibraryModel struct {
	lists                []list.Model
	removeList           list.Model
	confirmList          list.Model
	width                int
	height               int
	tabs                 []string
	activeTab            int
	discoveryInput       textinput.Model
	searchQuery          string
	searchLoading        bool
	scrapeLoading        bool
	searchStatusKind     searchStatusKind
	searchStatusCount    int
	searchStatusTitle    string
	scrapeTitle          string
	searchErr            error
	settingsStatusKind   settingsStatusKind
	settingsStatusErr    error
	settingsBusy         bool
	settingsState        settingsState
	pendingRemovePath    string
	pendingRemoveNovel   *library.Novel
	removeMode           SettingKind
	confirmPrompt        string
	language             lang.Locale
	localByRoot          map[string][]library.Novel
	onlineNovels         []library.Novel
	bookshelfRootItems   []BookshelfItem
	bookshelfInFolder    bool
	bookshelfInDiscovery bool
	bookshelfActiveRoot  string
}

type BookshelfItemKind int

const (
	BookshelfItemFolder BookshelfItemKind = iota
	BookshelfItemDiscovery
)

type BookshelfItem struct {
	Kind   BookshelfItemKind
	Name   string
	Path   string
	Novels []library.Novel
}

func (b BookshelfItem) Title() string {
	switch b.Kind {
	case BookshelfItemFolder:
		return b.Name
	case BookshelfItemDiscovery:
		return b.Name
	default:
		return b.Name
	}
}

func (b BookshelfItem) Description() string {
	return b.Path
}

func (b BookshelfItem) FilterValue() string {
	switch b.Kind {
	case BookshelfItemDiscovery:
		var names []string
		for _, n := range b.Novels {
			names = append(names, n.Name)
		}
		return b.Name + " " + strings.Join(names, " ")
	}
	return b.Name + " " + b.Path
}

type SettingKind int

const (
	SettingLineSpacing SettingKind = iota
	SettingLanguage
	SettingAddLibraryFolder
	SettingRemoveLibraryFolder
	SettingRemoveOnlineNovel
)

type settingsState int

const (
	settingsStateNormal settingsState = iota
	settingsStateRemoving
	settingsStateConfirm
)

type searchStatusKind int

const (
	searchStatusNone searchStatusKind = iota
	searchStatusFound
	searchStatusLoadingTitle
	searchStatusLoadingGeneric
	searchStatusSearching
)

type settingsStatusKind int

const (
	settingsStatusNone settingsStatusKind = iota
	settingsStatusLineSpacingFailed
	settingsStatusDialogUnavailable
	settingsStatusSaveFailed
)

const maxLineSpacing = 5

type SettingItem struct {
	Kind     SettingKind
	Label    string
	Detail   string
	Value    string
	IntValue int
	Extra    string
}

func (s SettingItem) Title() string {
	switch s.Kind {
	case SettingLineSpacing:
		return fmt.Sprintf("%s: %d", s.Label, s.IntValue)
	case SettingLanguage:
		if strings.TrimSpace(s.Value) == "" {
			return s.Label
		}
		return fmt.Sprintf("%s: %s", s.Label, s.Value)
	case SettingAddLibraryFolder, SettingRemoveLibraryFolder, SettingRemoveOnlineNovel:
		suffix := settingCountSuffix(s.IntValue)
		if suffix == "" {
			return s.Label
		}
		return fmt.Sprintf("%s %s", s.Label, suffix)
	default:
		return s.Label
	}
}

func (s SettingItem) Description() string {
	if s.Kind == SettingLanguage {
		return s.Detail
	}
	if strings.TrimSpace(s.Extra) == "" {
		return s.Detail
	}
	return strings.TrimSpace(s.Detail + " " + s.Extra)
}

func (s SettingItem) FilterValue() string {
	parts := []string{s.Label, s.Detail, s.Extra, s.Value}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func settingCountSuffix(count int) string {
	texts := lang.Active()
	switch count {
	case 0:
		return texts.Settings.CountSuffixNone
	case 1:
		return texts.Settings.CountSuffixSingle
	default:
		return fmt.Sprintf(texts.Settings.CountSuffixMultiple, count)
	}
}

func (m LibraryModel) searchStatusText() string {
	switch m.searchStatusKind {
	case searchStatusFound:
		return lang.SearchFound(m.searchStatusCount)
	case searchStatusLoadingTitle:
		if strings.TrimSpace(m.searchStatusTitle) != "" {
			return lang.SearchLoadingTitle(m.searchStatusTitle)
		}
		return lang.Active().Search.LoadingGeneric
	case searchStatusLoadingGeneric:
		return lang.Active().Search.LoadingGeneric
	case searchStatusSearching:
		return lang.Active().Search.Searching
	default:
		return ""
	}
}

func (m LibraryModel) settingsStatusText() string {
	switch m.settingsStatusKind {
	case settingsStatusLineSpacingFailed:
		if m.settingsStatusErr != nil {
			return fmt.Sprintf(lang.Active().Settings.LineSpacingUpdateFailed, m.settingsStatusErr)
		}
	case settingsStatusDialogUnavailable:
		return lang.Active().Settings.FolderDialogUnavailable
	case settingsStatusSaveFailed:
		if m.settingsStatusErr != nil {
			return fmt.Sprintf(lang.Active().Settings.SaveConfigFailed, m.settingsStatusErr)
		}
	}
	return ""
}

func (m *LibraryModel) clearSearchStatus() {
	m.searchStatusKind = searchStatusNone
	m.searchStatusCount = 0
	m.searchStatusTitle = ""
}

func (m *LibraryModel) rebuildSettingsList() {
	if len(m.lists) < 4 {
		return
	}
	texts := lang.Active()
	languageName := lang.LanguageName(m.language)

	settings := m.lists[3]
	var selectedKind SettingKind = -1
	previousIndex := settings.Index()
	if item, ok := settings.SelectedItem().(SettingItem); ok {
		selectedKind = item.Kind
	}

	items := []list.Item{
		SettingItem{
			Kind:   SettingLanguage,
			Label:  texts.Settings.LanguageLabel,
			Detail: texts.Settings.LanguageDetail,
			Value:  languageName,
		},
		SettingItem{
			Kind:     SettingLineSpacing,
			Label:    texts.Settings.LineSpacingLabel,
			Detail:   texts.Settings.LineSpacingDetail,
			IntValue: utils.AppConfig.Reader.LineSpacing,
		},
		SettingItem{
			Kind:     SettingAddLibraryFolder,
			Label:    texts.Settings.AddFolderLabel,
			Detail:   texts.Settings.AddFolderDetail,
			IntValue: len(utils.AppConfig.Library.Paths),
		},
		SettingItem{
			Kind:     SettingRemoveLibraryFolder,
			Label:    texts.Settings.RemoveFolderLabel,
			Detail:   texts.Settings.RemoveFolderDetail,
			IntValue: len(utils.AppConfig.Library.Paths),
		},
		SettingItem{
			Kind:     SettingRemoveOnlineNovel,
			Label:    texts.Settings.RemoveOnlineLabel,
			Detail:   texts.Settings.RemoveOnlineDetail,
			IntValue: len(m.onlineNovels),
		},
	}

	settings.SetItems(items)

	selectedSet := false
	if selectedKind != -1 {
		for i, item := range items {
			if s, ok := item.(SettingItem); ok && s.Kind == selectedKind {
				settings.Select(i)
				selectedSet = true
				break
			}
		}
	}
	if !selectedSet && previousIndex >= 0 && previousIndex < len(items) {
		settings.Select(previousIndex)
		selectedSet = true
	}
	if !selectedSet && len(items) > 0 {
		settings.Select(0)
	}

	m.lists[3] = settings
}

func (m *LibraryModel) updateConfirmPrompt() {
	if m.settingsState != settingsStateConfirm {
		return
	}
	texts := lang.Active()
	var prompt string
	var label string
	switch m.removeMode {
	case SettingRemoveLibraryFolder:
		prompt = fmt.Sprintf(texts.Confirm.RemoveFolderPromptTemplate, m.pendingRemovePath)
		label = texts.Confirm.RemoveFolderConfirm
	case SettingRemoveOnlineNovel:
		name := ""
		if m.pendingRemoveNovel != nil {
			name = m.pendingRemoveNovel.Name
		}
		prompt = fmt.Sprintf(texts.Confirm.RemoveOnlinePromptTemplate, name)
		label = texts.Confirm.RemoveOnlineConfirm
	default:
		return
	}

	_, contentW := m.confirmDialogWidths()
	wrapped := wordwrap.String(prompt, contentW)
	m.confirmPrompt = wrapped
	choices := []list.Item{
		confirmChoice{Label: label, Value: true},
	}
	m.confirmList.SetItems(choices)
	m.confirmList.SetSize(contentW, 2)
	m.confirmList.Select(0)
}

func (m *LibraryModel) applyLanguage() {
	texts := lang.Active()
	m.tabs = []string{
		texts.Tabs.History,
		texts.Tabs.Library,
		texts.Tabs.Discovery,
		texts.Tabs.Settings,
	}
	m.discoveryInput.Prompt = texts.Search.Prompt
	m.discoveryInput.Placeholder = texts.Search.Placeholder

	for i := range m.bookshelfRootItems {
		if m.bookshelfRootItems[i].Kind == BookshelfItemDiscovery {
			m.bookshelfRootItems[i].Name = texts.Bookshelf.DiscoveryName
		}
	}
	if len(m.lists) > 1 {
		libraryList := m.lists[1]
		for i, item := range libraryList.Items() {
			if bi, ok := item.(BookshelfItem); ok && bi.Kind == BookshelfItemDiscovery {
				bi.Name = texts.Bookshelf.DiscoveryName
				libraryList.SetItem(i, bi)
			}
		}
		m.lists[1] = libraryList
	}
	for i := range m.lists {
		l := m.lists[i]
		l.FilterInput.Prompt = texts.Search.Prompt
		m.lists[i] = l
	}
	remove := m.removeList
	remove.FilterInput.Prompt = texts.Search.Prompt
	m.removeList = remove

	if m.settingsState == settingsStateConfirm {
		m.updateConfirmPrompt()
	}
}

func (m *LibraryModel) changeLanguage(delta int) tea.Cmd {
	locales := lang.AvailableLocales()
	if len(locales) == 0 {
		return nil
	}
	currentIdx := 0
	for i, loc := range locales {
		if loc == m.language {
			currentIdx = i
			break
		}
	}
	nextIdx := (currentIdx + delta) % len(locales)
	if nextIdx < 0 {
		nextIdx += len(locales)
	}
	newLocale := locales[nextIdx]
	if newLocale == m.language {
		return nil
	}
	if !lang.SetLocale(newLocale) {
		return nil
	}
	previousLocale := m.language
	previousConfigLang := utils.AppConfig.UI.Language
	if previousConfigLang != string(newLocale) {
		utils.AppConfig.UI.Language = string(newLocale)
		if err := utils.SaveConfig(); err != nil {
			utils.AppConfig.UI.Language = previousConfigLang
			_ = lang.SetLocale(previousLocale)
			m.language = previousLocale
			m.rebuildSettingsList()
			m.applyLanguage()
			m.settingsStatusKind = settingsStatusSaveFailed
			m.settingsStatusErr = err
			return nil
		}
	}
	m.language = newLocale
	m.settingsStatusKind = settingsStatusNone
	m.settingsStatusErr = nil
	m.rebuildSettingsList()
	m.applyLanguage()
	return func() tea.Msg { return languageChangedMsg{Locale: newLocale} }
}

type LibraryPathItem struct {
	Name string
	Path string
}

func (i LibraryPathItem) Title() string {
	return i.Name
}

func (i LibraryPathItem) Description() string {
	return i.Path
}

func (i LibraryPathItem) FilterValue() string {
	return i.Name + " " + i.Path
}

type confirmChoice struct {
	Label string
	Value bool
}

func (c confirmChoice) Title() string {
	return c.Label
}

func (c confirmChoice) Description() string {
	return ""
}

func (c confirmChoice) FilterValue() string {
	return c.Label
}

func (m *LibraryModel) showBookshelfRoot() {
	m.bookshelfInFolder = false
	m.bookshelfInDiscovery = false
	m.bookshelfActiveRoot = ""
	items := make([]list.Item, len(m.bookshelfRootItems))
	for i := range m.bookshelfRootItems {
		items[i] = m.bookshelfRootItems[i]
	}
	m.lists[1].SetItems(items)
	if len(items) > 0 {
		m.lists[1].Select(0)
	}
	m.updateBookshelfSize()
}

func (m *LibraryModel) showBookshelfFolder(path string) {
	m.bookshelfInFolder = true
	m.bookshelfInDiscovery = false
	m.bookshelfActiveRoot = path

	novels := append([]library.Novel(nil), m.localByRoot[path]...)
	items := make([]list.Item, len(novels))
	for i := range novels {
		items[i] = novels[i]
	}
	m.lists[1].SetItems(items)
	if len(items) > 0 {
		m.lists[1].Select(0)
	}
	m.updateBookshelfSize()
}

func (m *LibraryModel) showBookshelfDiscovery(item BookshelfItem) {
	m.bookshelfInFolder = false
	m.bookshelfInDiscovery = true
	m.bookshelfActiveRoot = item.Path

	novels := append([]library.Novel(nil), item.Novels...)
	if refreshed, err := library.LoadAllCachedNovels(); err == nil {
		novels = refreshed
	} else if !os.IsNotExist(err) && err != nil {
		fmt.Println("Failed to load cached novels:", err)
	}
	sortOnlineNovels(novels)
	m.onlineNovels = novels
	m.updateSettingItem(SettingRemoveOnlineNovel, func(s *SettingItem) {
		s.IntValue = len(novels)
	})

	for i := range m.bookshelfRootItems {
		if m.bookshelfRootItems[i].Kind == BookshelfItemDiscovery {
			m.bookshelfRootItems[i].Novels = append([]library.Novel(nil), novels...)
			break
		}
	}

	items := make([]list.Item, len(novels))
	for i := range novels {
		items[i] = novels[i]
	}
	m.lists[1].SetItems(items)
	if len(items) > 0 {
		m.lists[1].Select(0)
	}
	m.updateBookshelfSize()
}

func sortOnlineNovels(novels []library.Novel) {
	sort.SliceStable(novels, func(i, j int) bool {
		ti := novels[i].Added
		tj := novels[j].Added
		if ti.IsZero() {
			ti = novels[i].Modified
		}
		if tj.IsZero() {
			tj = novels[j].Modified
		}
		return ti.After(tj)
	})
}

func (m *LibraryModel) selectBookshelfPath(path string) {
	if path == "" {
		if len(m.bookshelfRootItems) > 0 {
			m.lists[1].Select(0)
		}
		return
	}

	items := m.lists[1].Items()
	for i, item := range items {
		if bi, ok := item.(BookshelfItem); ok && bi.Path == path {
			m.lists[1].Select(i)
			return
		}
	}

	if len(items) > 0 {
		m.lists[1].Select(0)
	}
}

func (m *LibraryModel) refreshHistoryList(progress map[string]utils.Progress) {
	if len(m.lists) == 0 {
		return
	}
	history := m.lists[0]
	items := history.Items()

	novels := make([]library.Novel, 0, len(items))
	for _, item := range items {
		n, ok := item.(library.Novel)
		if !ok {
			continue
		}

		source := "local"
		if !n.IsLocal {
			source = "online"
		}
		if p, ok := utils.GetProgress(progress, n.Name, source); ok {
			if !p.LastRead.IsZero() {
				n.Modified = p.LastRead
			}
			if p.LastChapter != "" {
				n.Current = p.LastChapter
			}
		}

		if n.IsLocal {
			if latest, err := library.LatestChapter(n.Path); err == nil && latest != "" {
				n.Latest = latest
			}
		} else {
			if chapters, err := library.LoadChapterList(n.Name); err == nil && len(chapters) > 0 {
				last := chapters[len(chapters)-1]
				latestTitle := strings.TrimSpace(last.Title)
				if latestTitle == "" {
					latestTitle = lang.ChapterTitle(last.Index)
				}
				n.Latest = latestTitle
			}
		}

		novels = append(novels, n)
	}

	sortNovelsByLastRead(novels, progress)

	newItems := make([]list.Item, len(novels))
	for i := range novels {
		newItems[i] = novels[i]
	}

	history.SetItems(newItems)
	if len(newItems) > 0 {
		history.Select(0)
	}
	m.lists[0] = history
}

func (m *LibraryModel) updateBookshelfSize() {
	if len(m.lists) <= 1 {
		return
	}
	availWidth := m.width - 8
	if availWidth > ListMaxWidth {
		availWidth = ListMaxWidth
	}
	if availWidth < 0 {
		availWidth = ListMaxWidth
	}
	availHeight := m.height - 5
	if availHeight < 3 {
		availHeight = 3
	}
	m.lists[1].SetSize(availWidth, availHeight)
}

func (m LibraryModel) handleBookshelfEnter() (LibraryModel, tea.Cmd) {
	selected := m.lists[1].SelectedItem()
	if selected == nil {
		return m, nil
	}

	switch v := selected.(type) {
	case BookshelfItem:
		switch v.Kind {
		case BookshelfItemFolder:
			return m, func() tea.Msg {
				return bookshelfNavigateMsg{Item: v}
			}
		case BookshelfItemDiscovery:
			return m, func() tea.Msg {
				return bookshelfNavigateMsg{Item: v}
			}
		}
	case library.Novel:
		novel := v
		return m, func() tea.Msg {
			return novelOpenMsg{Novel: novel}
		}
	}

	return m, nil
}

// ----- Tab navigation -----
func (m *LibraryModel) nextTab() {
	m.activeTab++
	if m.activeTab >= len(m.tabs) {
		m.activeTab = 0
	}
}

func (m *LibraryModel) prevTab() {
	m.activeTab--
	if m.activeTab < 0 {
		m.activeTab = len(m.tabs) - 1
	}
}

func (m *LibraryModel) resize(width, height int) {
	m.width = width
	m.height = height

	availWidth := width - 8
	if availWidth > ListMaxWidth {
		availWidth = ListMaxWidth
	}
	if availWidth < 0 {
		availWidth = ListMaxWidth
	}
	availHeight := height - 5
	if availHeight < 3 {
		availHeight = 3
	}
	for i := range m.lists {
		m.lists[i].SetSize(availWidth, availHeight)
	}
	m.removeList.SetSize(availWidth, availHeight)
	confirmWidth := availWidth
	if confirmWidth > 40 {
		confirmWidth = 40
	}
	if confirmWidth < 20 {
		confirmWidth = availWidth
	}
	m.confirmList.SetSize(confirmWidth, 5)

	if m.settingsState == settingsStateConfirm {
		_, contentW := m.confirmDialogWidths()
		m.confirmList.SetSize(contentW, 2)
	}
}

// ---------------- Update ----------------
func (m LibraryModel) Init() tea.Cmd { return nil }

type searchMsg struct {
	Items []list.Item
	Query string
	Err   error
}

type scrapeMsg struct {
	Title string
	Err   error
}

type errMsg struct{ error }

type bookshelfNavigateMsg struct {
	Item BookshelfItem
}

type folderSelectedMsg struct {
	Path string
	Err  error
}

type languageChangedMsg struct {
	Locale lang.Locale
}

func (m LibraryModel) handleSearchMsg(tm searchMsg) (LibraryModel, tea.Cmd) {
	if tm.Query != m.searchQuery {
		return m, nil
	}
	if tm.Err != nil {
		m.searchErr = tm.Err
		m.searchLoading = false
		m.clearSearchStatus()
		return m, nil
	}

	availWidth := m.width - 8
	if availWidth > ListMaxWidth {
		availWidth = ListMaxWidth
	}
	availHeight := m.height - 4

	discoveryList := list.New(tm.Items, &NovelDelegate{}, availWidth, availHeight)
	listSettings(&discoveryList)
	filterStyle(&discoveryList)
	if len(tm.Items) > 0 {
		discoveryList.Select(0)
	}
	m.lists[2] = discoveryList
	m.searchLoading = false
	m.searchQuery = tm.Query
	m.searchErr = nil
	m.searchStatusKind = searchStatusFound
	m.searchStatusCount = len(tm.Items)
	m.searchStatusTitle = ""
	m.discoveryInput.SetValue("")
	return m, nil
}

func (m LibraryModel) Update(msg tea.Msg) (LibraryModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch tm := msg.(type) {

	case tea.KeyMsg:
		key := tm.String()

		// --- Global keys ---
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.activeTab == 3 {
				switch m.settingsState {
				case settingsStateRemoving:
					if m.removeList.FilterState() == list.Filtering || m.removeList.IsFiltered() {
						newList, c := m.removeList.Update(tm)
						m.removeList = newList
						return m, c
					}
				case settingsStateConfirm:
					if m.confirmList.FilterState() == list.Filtering || m.confirmList.IsFiltered() {
						newList, c := m.confirmList.Update(tm)
						m.confirmList = newList
						return m, c
					}
				}
			}
			if m.activeTab >= 0 && m.activeTab < len(m.lists) {
				if m.lists[m.activeTab].FilterState() == list.Filtering || m.lists[m.activeTab].IsFiltered() {
					newList, c := m.lists[m.activeTab].Update(tm)
					m.lists[m.activeTab] = newList
					return m, c
				}
			}
			if m.activeTab == 3 {
				switch m.settingsState {
				case settingsStateConfirm:
					m.settingsState = settingsStateRemoving
					m.confirmPrompt = ""
					m.pendingRemovePath = ""
					m.pendingRemoveNovel = nil
					//m.settingsStatus = "Removal cancelled."
					return m, nil
				case settingsStateRemoving:
					m.settingsStatusKind = settingsStatusNone
					m.settingsStatusErr = nil
					m.exitRemoveMode()
					return m, nil
				}
			}
			if m.activeTab == 1 && (m.bookshelfInFolder || m.bookshelfInDiscovery) {
				prevPath := m.bookshelfActiveRoot
				m.showBookshelfRoot()
				if prevPath != "" {
					m.selectBookshelfPath(prevPath)
				}
				return m, nil
			}
			if m.activeTab == 2 && (m.discoveryInput.Value() != "" || m.searchQuery != "" || m.searchLoading || m.scrapeLoading) {
				m.searchLoading = false
				m.scrapeLoading = false
				m.scrapeTitle = ""
				m.clearSearchStatus()
				m.searchQuery = ""
				m.discoveryInput.SetValue("")
				m.lists[2].SetItems(nil)
				m.searchErr = nil
				return m, nil
			}
			return m, nil
		case "tab":
			m.nextTab()
			return m, nil
		case "shift+tab":
			m.prevTab()
			return m, nil
		case "h", "left":
			if m.activeTab != 3 {
				newList, c := m.lists[m.activeTab].Update(tea.KeyMsg{Type: tea.KeyPgUp})
				m.lists[m.activeTab] = newList
				return m, c
			}
		case "l", "right":
			if m.activeTab != 3 {
				newList, c := m.lists[m.activeTab].Update(tea.KeyMsg{Type: tea.KeyPgDown})
				m.lists[m.activeTab] = newList
				return m, c
			}
		}

		if m.activeTab == 1 {
			switch key {
			case "backspace", "delete", "ctrl+h":
				if m.bookshelfInFolder || m.bookshelfInDiscovery {
					prevPath := m.bookshelfActiveRoot
					m.showBookshelfRoot()
					if prevPath != "" {
						m.selectBookshelfPath(prevPath)
					}
					return m, nil
				}
			case "enter":
				return m.handleBookshelfEnter()
			}
		}

		// --- Discovery tab handling ---
		if m.activeTab == 2 {
			if m.scrapeLoading {
				return m, nil
			}
			// Navigation in results
			if len(m.lists[2].Items()) > 0 {
				switch key {
				case "j", "down":
					newList, c := m.lists[2].Update(tea.KeyMsg{Type: tea.KeyDown})
					m.lists[2] = newList
					return m, c
				case "k", "up":
					newList, c := m.lists[2].Update(tea.KeyMsg{Type: tea.KeyUp})
					m.lists[2] = newList
					return m, c
				case "enter":
					selected := m.lists[2].SelectedItem()
					if sr, ok := selected.(library.SearchResult); ok {
						m.scrapeLoading = true
						m.scrapeTitle = sr.Name
						m.searchStatusKind = searchStatusLoadingTitle
						m.searchStatusTitle = sr.Name
						m.searchStatusCount = 0
						m.searchErr = nil

						// Add to History immediately
						historyList := m.lists[0]
						novel := library.Novel{
							Name:      sr.Name,
							Author:    sr.Author,
							OnlineURL: sr.URL,
							Modified:  time.Now(),
							Added:     time.Now(),
						}
						historyList.InsertItem(0, novel)
						m.lists[0] = historyList

						// Async scrape
						return m, func() tea.Msg {
							novel, _ := library.ScrapeAndSaveChapters(sr)
							return novelOpenMsg{Novel: novel}
						}
					}
				}

				newList, c := m.lists[2].Update(tm)
				m.lists[2] = newList
				return m, c
			}

			if !m.scrapeLoading {
				ti, c := m.discoveryInput.Update(tm)
				m.discoveryInput = ti
				if c != nil {
					cmds = append(cmds, c)
				}

				if key != "enter" {
					m.searchQuery = m.discoveryInput.Value()
				}

				if key == "enter" {
					query := strings.TrimSpace(m.discoveryInput.Value())
					if query != "" && !m.searchLoading {
						m.searchQuery = query
						m.searchLoading = true
						m.scrapeLoading = false
						m.scrapeTitle = ""
						m.searchErr = nil
						m.searchStatusKind = searchStatusSearching
						m.searchStatusTitle = ""
						m.searchStatusCount = 0
						m.lists[2].SetItems(nil)
						m.discoveryInput.SetValue("")

						return m, func() tea.Msg {
							items, err := library.SearchNovel(query)
							return searchMsg{Items: items, Query: query, Err: err}
						}
					}
				}
			}

			// ignore other keys on discovery tab
			return m, tea.Batch(cmds...)
		}

		if m.activeTab == 3 {
			switch m.settingsState {
			case settingsStateNormal:
				selected := m.lists[3].SelectedItem()
				switch key {
				case "j", "down":
					newList, c := m.lists[3].Update(tea.KeyMsg{Type: tea.KeyDown})
					m.lists[3] = newList
					return m, c
				case "k", "up":
					newList, c := m.lists[3].Update(tea.KeyMsg{Type: tea.KeyUp})
					m.lists[3] = newList
					return m, c
				case "h", "left":
					if s, ok := selected.(SettingItem); ok {
						switch s.Kind {
						case SettingLineSpacing:
							_, err := m.changeLineSpacing(-1)
							if err != nil {
								m.settingsStatusKind = settingsStatusLineSpacingFailed
								m.settingsStatusErr = err
							} else {
								m.settingsStatusKind = settingsStatusNone
								m.settingsStatusErr = nil
							}
							return m, nil
						case SettingLanguage:
							if cmd := m.changeLanguage(-1); cmd != nil {
								return m, cmd
							}
							return m, nil
						}
					}
				case "l", "right":
					if s, ok := selected.(SettingItem); ok {
						switch s.Kind {
						case SettingLineSpacing:
							_, err := m.changeLineSpacing(1)
							if err != nil {
								m.settingsStatusKind = settingsStatusLineSpacingFailed
								m.settingsStatusErr = err
							} else {
								m.settingsStatusKind = settingsStatusNone
								m.settingsStatusErr = nil
							}
							return m, nil
						case SettingLanguage:
							if cmd := m.changeLanguage(1); cmd != nil {
								return m, cmd
							}
							return m, nil
						}
					}
				case "enter":
					if s, ok := selected.(SettingItem); ok {
						switch s.Kind {
						case SettingAddLibraryFolder:
							if m.settingsBusy {
								return m, nil
							}
							m.settingsBusy = true
							return m, selectFolderCmd()
						case SettingRemoveLibraryFolder:
							if m.settingsBusy {
								return m, nil
							}
							m.enterRemoveMode(SettingRemoveLibraryFolder)
							return m, nil
						case SettingRemoveOnlineNovel:
							if m.settingsBusy {
								return m, nil
							}
							m.enterRemoveMode(SettingRemoveOnlineNovel)
							return m, nil
						}
					}
					return m, nil
				}

				newList, c := m.lists[3].Update(tm)
				m.lists[3] = newList
				return m, c

			case settingsStateRemoving:
				switch key {
				case "j", "down":
					newList, c := m.removeList.Update(tea.KeyMsg{Type: tea.KeyDown})
					m.removeList = newList
					return m, c
				case "k", "up":
					newList, c := m.removeList.Update(tea.KeyMsg{Type: tea.KeyUp})
					m.removeList = newList
					return m, c
				case "h", "left", "backspace", "delete":
					m.settingsStatusKind = settingsStatusNone
					m.settingsStatusErr = nil
					m.exitRemoveMode()
					return m, nil
				case "enter":
					switch m.removeMode {
					case SettingRemoveLibraryFolder:
						if item, ok := m.removeList.SelectedItem().(LibraryPathItem); ok {
							m.beginRemoveLibraryConfirm(item)
						}
					case SettingRemoveOnlineNovel:
						if item, ok := m.removeList.SelectedItem().(library.Novel); ok {
							m.beginRemoveOnlineConfirm(item)
						}
					}
					return m, nil
				}
				newList, c := m.removeList.Update(tm)
				m.removeList = newList
				return m, c

			case settingsStateConfirm:
				switch key {
				case "j", "down":
					idx := m.confirmList.Index()
					if idx < len(m.confirmList.Items())-1 {
						m.confirmList.Select(idx + 1)
					}
					return m, nil
				case "k", "up":
					idx := m.confirmList.Index()
					if idx > 0 {
						m.confirmList.Select(idx - 1)
					}
					return m, nil
				case "enter":
					if ch, ok := m.confirmList.SelectedItem().(confirmChoice); ok {
						if ch.Value {
							switch m.removeMode {
							case SettingRemoveLibraryFolder:
								if err := m.removeLibraryPath(m.pendingRemovePath); err != nil {
									// handle error & bounce back
								}
							case SettingRemoveOnlineNovel:
								if m.pendingRemoveNovel != nil {
									if err := m.removeOnlineNovel(m.pendingRemoveNovel.Name); err != nil {
										// handle error & bounce back
									}
								}
							}
							m.confirmPrompt = ""
							m.pendingRemovePath = ""
							m.pendingRemoveNovel = nil
							if len(m.removeList.Items()) == 0 {
								m.exitRemoveMode()
							} else {
								m.settingsState = settingsStateRemoving
							}
							return m, nil
						}
						// cancel
						m.settingsState = settingsStateRemoving
						m.confirmPrompt = ""
						m.pendingRemovePath = ""
						m.pendingRemoveNovel = nil
						return m, nil
					}
				case "esc":
					m.settingsState = settingsStateRemoving
					m.confirmPrompt = ""
					m.pendingRemovePath = ""
					m.pendingRemoveNovel = nil
					return m, nil
				}
				newList, c := m.confirmList.Update(tm)
				m.confirmList = newList
				return m, c
			}
		}

		// --- Other tabs (local/history) ---
		switch key {
		case "j", "down":
			newList, c := m.lists[m.activeTab].Update(tea.KeyMsg{Type: tea.KeyDown})
			m.lists[m.activeTab] = newList
			return m, c
		case "k", "up":
			newList, c := m.lists[m.activeTab].Update(tea.KeyMsg{Type: tea.KeyUp})
			m.lists[m.activeTab] = newList
			return m, c
		case "enter":
			selected := m.lists[m.activeTab].SelectedItem()
			if selected == nil {
				return m, nil
			}

			if novel, ok := selected.(library.Novel); ok {
				return m, func() tea.Msg {
					// just send novelOpenMsg; AppModel creates ReaderModel
					return novelOpenMsg{Novel: novel}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.resize(tm.Width, tm.Height)
		return m, nil

	case novelOpenMsg:
		m.handleNovelOpened(tm.Novel)
		return m, nil

	case searchMsg:
		return m.handleSearchMsg(tm)

	case folderSelectedMsg:
		m.settingsBusy = false
		if tm.Err != nil {
			switch {
			case errors.Is(tm.Err, utils.ErrDialogCancelled):
				m.settingsStatusKind = settingsStatusNone
				m.settingsStatusErr = nil
			case errors.Is(tm.Err, utils.ErrDialogUnavailable):
				m.settingsStatusKind = settingsStatusDialogUnavailable
				m.settingsStatusErr = nil
			default:
				//m.settingsStatus = fmt.Sprintf("")
			}
			return m, nil
		}

		if strings.TrimSpace(tm.Path) == "" {
			m.settingsStatusKind = settingsStatusNone
			m.settingsStatusErr = nil
			return m, nil
		}

		if err := m.addLibraryPath(tm.Path); err != nil {
			//m.settingsStatus = fmt.Sprintf("Failed to add folder: %v", err)
			return m, nil
		}

		m.settingsStatusKind = settingsStatusNone
		m.settingsStatusErr = nil
		return m, nil

	case scrapeMsg:
		m.searchLoading = false
		m.scrapeLoading = false
		if tm.Err != nil {
			m.searchErr = tm.Err
			m.clearSearchStatus()
			fmt.Println("Error scraping novel:", tm.Err)
		}
		return m, nil

	case bookshelfNavigateMsg:
		switch tm.Item.Kind {
		case BookshelfItemFolder:
			m.showBookshelfFolder(tm.Item.Path)
		case BookshelfItemDiscovery:
			m.showBookshelfDiscovery(tm.Item)
		}
		return m, nil

	case errMsg:
		m.searchErr = tm.error
		m.searchLoading = false
		m.clearSearchStatus()
		return m, nil
	}

	// Default: delegate to active list
	newList, cmd := m.lists[m.activeTab].Update(msg)
	m.lists[m.activeTab] = newList
	return m, cmd
}

// ---------------- View ----------------
func (m LibraryModel) View() string {
	var renderedTabs []string
	for i, name := range m.tabs {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, ActiveTabStyle.Render(name))
		} else {
			renderedTabs = append(renderedTabs, InactiveTabStyle.Render(name))
		}
	}
	texts := lang.Active()
	tabsRow := TabsRow.Width(m.width).Render(gloss.JoinHorizontal(gloss.Top, renderedTabs...))

	maxUnderline := texts.Layout.UnderlineLength
	if maxUnderline <= 0 {
		maxUnderline = 48
	}
	lineWidth := m.width
	if lineWidth > maxUnderline {
		lineWidth = maxUnderline
	}
	underlineRow := UnderlineRow.Width(m.width).Render(strings.Repeat("─", lineWidth))

	if m.activeTab == 2 {
		showInput := len(m.lists[2].Items()) == 0 && !m.searchLoading && !m.scrapeLoading && m.searchStatusKind == searchStatusNone
		inputView := ""
		if showInput {
			inputView = gloss.Place(
				m.width,
				4,
				gloss.Center,
				gloss.Center,
				PromptBoxStyle.Render(m.discoveryInput.View()),
			)
		}

		statusText := ""
		switch {
		case m.scrapeLoading:
			if m.scrapeTitle != "" {
				statusText = lang.SearchLoadingTitle(m.scrapeTitle)
			} else {
				statusText = texts.Search.LoadingGeneric
			}
		case m.searchLoading:
			statusText = texts.Search.Searching
		case m.searchErr != nil:
			statusText = lang.SearchError(m.searchErr)
		case m.searchStatusKind != searchStatusNone:
			statusText = m.searchStatusText()
		}

		statusView := ""
		if statusText != "" {
			statusView = StatusStyle.Width(m.width).Render(statusText)
		}

		listView := ""
		if len(m.lists[2].Items()) > 0 {
			containerWidth := ListMaxWidth
			if m.width < containerWidth {
				containerWidth = m.width - 8
			}
			if containerWidth < 0 {
				containerWidth = ListMaxWidth
			}
			listBlock := ListStyle.Width(containerWidth).Render(m.lists[2].View())
			listView = List.Width(m.width).Render(listBlock)
		} else if !showInput && statusText == "" && !m.searchLoading && !m.scrapeLoading {
			listView = StatusMutedStyle.Width(m.width).Render(texts.Search.InputHint)
		}
		result := tabsRow + "\n" + underlineRow
		if inputView != "" {
			result += "\n" + inputView
		}
		if statusView != "" {
			result += "\n" + statusView
		}
		if listView != "" {
			result += "\n" + listView
		}
		return result
	}

	containerWidth := ListMaxWidth
	if m.width < containerWidth {
		containerWidth = m.width - 8
	}
	var listView string
	if m.activeTab == 3 && (m.settingsState == settingsStateRemoving || m.settingsState == settingsStateConfirm) {
		listBlock := ListStyle.Width(containerWidth).Render(m.removeList.View())
		listView = List.Width(m.width).Render(listBlock)
	} else {
		listBlock := ListStyle.Width(containerWidth).Render(m.lists[m.activeTab].View())
		listView = List.Width(m.width).Render(listBlock)
	}

	result := tabsRow + "\n" + underlineRow
	if m.activeTab == 3 {
		if status := strings.TrimSpace(m.settingsStatusText()); status != "" {
			result += "\n" + StatusStyle.Width(m.width).Render(status)
		}
	}
	result += listView
	if m.activeTab == 3 && m.settingsState == settingsStateConfirm {
		dlgW, contentW := m.confirmDialogWidths()

		// Re-wrap on late resize (wrap before styling; strip ANSI just in case)
		prompt := wordwrap.String(m.confirmPrompt, contentW)
		body := strings.Join([]string{
			ConfirmPromptStyle.Render(prompt),
			ConfirmListStyle.Width(contentW).Render(m.confirmList.View()),
		}, "\n\n")

		dialog := ConfirmBoxStyle.Width(dlgW).Render(body)

		overlay := gloss.Place(m.width, m.height, gloss.Center, gloss.Center, dialog)
		base := tabsRow + "\n" + underlineRow + listView
		return base + "\n" + overlay // no dimming
	}
	return result
}

// ---------------- NovelDelegate ----------------
type NovelDelegate struct {
	list.DefaultDelegate
}

func (d *NovelDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	var title, desc string
	switch v := item.(type) {
	case library.Novel:
		title = v.Title()
		desc = runewidth.Truncate(v.Description(), m.Width()-10, "…")
	case library.SearchResult:
		title = v.Title()
		desc = runewidth.Truncate(v.Description(), m.Width()-10, "…")
	case BookshelfItem:
		title = v.Title()
		desc = runewidth.Truncate(v.Description(), m.Width()-10, "…")
	case SettingItem:
		title = v.Title()
		desc = runewidth.Truncate(v.Description(), m.Width()-10, "…")
	case LibraryPathItem:
		title = v.Title()
		desc = runewidth.Truncate(v.Description(), m.Width()-10, "…")
	case confirmChoice:
		title = v.Title()
		desc = runewidth.Truncate(v.Description(), m.Width()-10, "…")
	default:
		title = lang.Active().Bookshelf.UnknownType
		desc = ""
	}
	if index == m.Index() {
		title = SelectedTitleStyle.Render(title)
		desc = SelectedDescStyle.Render(desc)
	} else {
		title = NormalTitleStyle.Render(title)
		desc = NormalDescStyle.Render(desc)
	}
	fmt.Fprintf(w, "%s\n%s", title, desc)
}

func (d *NovelDelegate) Height() int  { return 2 }
func (d *NovelDelegate) Spacing() int { return 1 }
func (d *NovelDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

// ---------------- List styling ----------------
func filterStyle(l *list.Model) {
	l.FilterInput.Prompt = lang.Active().Search.Prompt
	l.FilterInput.PromptStyle = PromptStyle
	l.FilterInput.TextStyle = PromptTextStyle
	l.FilterInput.Cursor.Style = PromptCursorStyle
}

func listSettings(l *list.Model) {
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()
}

// ---------------- Settings helpers ----------------
func selectFolderCmd() tea.Cmd {
	start := ""
	if len(utils.AppConfig.Library.Paths) > 0 {
		start = utils.AppConfig.Library.Paths[0]
	}
	return func() tea.Msg {
		path, err := utils.SelectFolderDialog(start)
		if err != nil {
			return folderSelectedMsg{Err: err}
		}
		return folderSelectedMsg{Path: path}
	}
}

func (m *LibraryModel) enterRemoveMode(kind SettingKind) {
	m.removeMode = kind
	switch kind {
	case SettingRemoveLibraryFolder:
		if len(utils.AppConfig.Library.Paths) == 0 {
			//m.settingsStatus = "No library folders to remove."
			return
		}
		m.refreshRemoveLibraryList("")
	case SettingRemoveOnlineNovel:
		if len(m.onlineNovels) == 0 {
			//m.settingsStatus = "No online novels to remove."
			return
		}
		m.refreshRemoveOnlineList("")
	default:
		return
	}
	m.settingsState = settingsStateRemoving
	m.settingsStatusKind = settingsStatusNone
	m.settingsStatusErr = nil
}

func (m *LibraryModel) refreshRemoveLibraryList(preferPath string) {
	var currentPath string
	if preferPath != "" {
		currentPath = filepath.Clean(preferPath)
	} else if selected, ok := m.removeList.SelectedItem().(LibraryPathItem); ok {
		currentPath = selected.Path
	}

	items := make([]list.Item, 0, len(utils.AppConfig.Library.Paths))
	for _, dir := range utils.AppConfig.Library.Paths {
		cleaned := filepath.Clean(dir)
		name := filepath.Base(cleaned)
		if name == "." || name == string(os.PathSeparator) {
			name = cleaned
		}
		items = append(items, LibraryPathItem{
			Name: name,
			Path: cleaned,
		})
	}

	m.removeList.SetItems(items)
	if len(items) == 0 {
		return
	}

	target := -1
	if currentPath != "" {
		for i, item := range items {
			if candidate, ok := item.(LibraryPathItem); ok && pathsEqual(candidate.Path, currentPath) {
				target = i
				break
			}
		}
	}

	if target >= 0 {
		m.removeList.Select(target)
	} else {
		m.removeList.Select(0)
	}
}

func (m *LibraryModel) refreshRemoveOnlineList(preferName string) {
	var currentName string
	if preferName != "" {
		currentName = preferName
	} else if selected, ok := m.removeList.SelectedItem().(library.Novel); ok {
		currentName = selected.Name
	}

	items := make([]list.Item, len(m.onlineNovels))
	for i := range m.onlineNovels {
		items[i] = m.onlineNovels[i]
	}

	m.removeList.SetItems(items)
	if len(items) == 0 {
		return
	}

	target := -1
	if currentName != "" {
		for i, item := range items {
			if novel, ok := item.(library.Novel); ok && novel.Name == currentName {
				target = i
				break
			}
		}
	}

	if target >= 0 {
		m.removeList.Select(target)
	} else {
		m.removeList.Select(0)
	}
}

func (m *LibraryModel) exitRemoveMode() {
	m.settingsState = settingsStateNormal
	m.pendingRemovePath = ""
	m.pendingRemoveNovel = nil
	m.confirmPrompt = ""
	switch m.removeMode {
	case SettingRemoveOnlineNovel:
		m.selectSettingItem(SettingRemoveOnlineNovel)
	default:
		m.selectSettingItem(SettingRemoveLibraryFolder)
	}
}

func (m *LibraryModel) beginRemoveLibraryConfirm(item LibraryPathItem) {
	m.pendingRemovePath = item.Path
	m.settingsStatusKind = settingsStatusNone
	m.settingsStatusErr = nil
	m.settingsState = settingsStateConfirm
	m.updateConfirmPrompt()
}

func (m *LibraryModel) beginRemoveOnlineConfirm(novel library.Novel) {
	m.pendingRemoveNovel = &novel
	m.settingsStatusKind = settingsStatusNone
	m.settingsStatusErr = nil
	m.settingsState = settingsStateConfirm
	m.updateConfirmPrompt()
}

func (m *LibraryModel) changeLineSpacing(delta int) (bool, error) {
	current := utils.AppConfig.Reader.LineSpacing
	newValue := current + delta
	if newValue < 0 {
		newValue = 0
	}
	if newValue > maxLineSpacing {
		newValue = maxLineSpacing
	}
	if newValue == current {
		return false, nil
	}

	utils.AppConfig.Reader.LineSpacing = newValue
	if err := utils.SaveConfig(); err != nil {
		utils.AppConfig.Reader.LineSpacing = current
		return false, err
	}

	m.updateSettingItem(SettingLineSpacing, func(s *SettingItem) {
		s.IntValue = newValue
	})

	return true, nil
}

func (m *LibraryModel) updateSettingItem(kind SettingKind, update func(*SettingItem)) {
	if len(m.lists) < 4 {
		return
	}
	settings := m.lists[3]
	for i, item := range settings.Items() {
		s, ok := item.(SettingItem)
		if !ok || s.Kind != kind {
			continue
		}
		update(&s)
		settings.SetItem(i, s)
		m.lists[3] = settings
		return
	}
}

func (m *LibraryModel) selectSettingItem(kind SettingKind) {
	if len(m.lists) < 4 {
		return
	}
	settings := m.lists[3]
	for i, item := range settings.Items() {
		s, ok := item.(SettingItem)
		if !ok || s.Kind != kind {
			continue
		}
		settings.Select(i)
		m.lists[3] = settings
		return
	}
}

func (m *LibraryModel) addLibraryPath(path string) error {
	cleaned := filepath.Clean(path)
	info, err := os.Stat(cleaned)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", cleaned)
	}

	for _, existing := range utils.AppConfig.Library.Paths {
		if pathsEqual(existing, cleaned) {
			return fmt.Errorf("folder already added")
		}
	}

	previous := append([]string(nil), utils.AppConfig.Library.Paths...)
	utils.AppConfig.Library.Paths = append(utils.AppConfig.Library.Paths, cleaned)

	if err := utils.SaveConfig(); err != nil {
		utils.AppConfig.Library.Paths = previous
		return err
	}

	if err := m.reloadLibraryFromConfig(); err != nil {
		utils.AppConfig.Library.Paths = previous
		if saveErr := utils.SaveConfig(); saveErr != nil {
			return fmt.Errorf("%v (rollback failed: %v)", err, saveErr)
		}
		if reloadErr := m.reloadLibraryFromConfig(); reloadErr != nil {
			return fmt.Errorf("%v (rollback reload failed: %v)", err, reloadErr)
		}
		return err
	}

	m.updateSettingItem(SettingAddLibraryFolder, func(s *SettingItem) {
		s.IntValue = len(utils.AppConfig.Library.Paths)
		//s.Extra = fmt.Sprintf("Last added: %s", cleaned)
	})
	m.updateSettingItem(SettingRemoveLibraryFolder, func(s *SettingItem) {
		s.IntValue = len(utils.AppConfig.Library.Paths)
	})
	m.refreshRemoveLibraryList(cleaned)

	return nil
}

func (m *LibraryModel) removeLibraryPath(path string) error {
	cleaned := filepath.Clean(path)
	index := -1
	for i, existing := range utils.AppConfig.Library.Paths {
		if pathsEqual(existing, cleaned) {
			index = i
			break
		}
	}
	if index == -1 {
		return fmt.Errorf("folder not found: %s", cleaned)
	}

	previous := append([]string(nil), utils.AppConfig.Library.Paths...)
	utils.AppConfig.Library.Paths = append(utils.AppConfig.Library.Paths[:index], utils.AppConfig.Library.Paths[index+1:]...)

	if err := utils.SaveConfig(); err != nil {
		utils.AppConfig.Library.Paths = previous
		return err
	}

	if err := m.reloadLibraryFromConfig(); err != nil {
		utils.AppConfig.Library.Paths = previous
		if saveErr := utils.SaveConfig(); saveErr != nil {
			return fmt.Errorf("%v (rollback failed: %v)", err, saveErr)
		}
		if reloadErr := m.reloadLibraryFromConfig(); reloadErr != nil {
			return fmt.Errorf("%v (rollback reload failed: %v)", err, reloadErr)
		}
		return err
	}

	newCount := len(utils.AppConfig.Library.Paths)
	m.updateSettingItem(SettingAddLibraryFolder, func(s *SettingItem) {
		s.IntValue = newCount
	})
	m.updateSettingItem(SettingRemoveLibraryFolder, func(s *SettingItem) {
		s.IntValue = newCount
		//s.Extra = fmt.Sprintf("Last removed: %s", cleaned)
	})
	m.refreshRemoveLibraryList("")

	return nil
}

func (m *LibraryModel) removeOnlineNovel(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("novel name is empty")
	}

	cachePath := library.NovelCachePath(name)
	if err := os.RemoveAll(cachePath); err != nil {
		return err
	}

	if err := utils.DeleteProgress(name, "online"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	novels, err := library.LoadAllCachedNovels()
	if err != nil {
		if os.IsNotExist(err) {
			novels = nil
		} else {
			return err
		}
	}

	sortOnlineNovels(novels)

	m.onlineNovels = novels
	m.updateSettingItem(SettingRemoveOnlineNovel, func(s *SettingItem) {
		s.IntValue = len(novels)
	})

	if err := m.reloadLibraryFromConfig(); err != nil {
		return err
	}

	if m.removeMode == SettingRemoveOnlineNovel {
		m.refreshRemoveOnlineList("")
	}

	return nil
}

func (m *LibraryModel) upsertOnlineNovel(novel library.Novel) {
	updated := false
	for i := range m.onlineNovels {
		if m.onlineNovels[i].Name == novel.Name {
			m.onlineNovels[i] = novel
			updated = true
			break
		}
	}
	if !updated {
		m.onlineNovels = append(m.onlineNovels, novel)
	}
	sortOnlineNovels(m.onlineNovels)
}

func (m *LibraryModel) upsertHistoryNovel(novel library.Novel) {
	if len(m.lists) == 0 {
		return
	}
	history := m.lists[0]
	items := history.Items()
	for i, item := range items {
		existing, ok := item.(library.Novel)
		if !ok {
			continue
		}
		if existing.Name == novel.Name && existing.IsLocal == novel.IsLocal {
			history.SetItem(i, novel)
			m.lists[0] = history
			return
		}
	}
	history.InsertItem(0, novel)
	history.Select(0)
	m.lists[0] = history
}

func (m *LibraryModel) handleNovelOpened(novel library.Novel) {
	if novel.IsLocal {
		progressMap, _ := utils.Load()
		m.refreshHistoryList(progressMap)
		return
	}

	if refreshed, err := library.LoadAllCachedNovels(); err == nil {
		sortOnlineNovels(refreshed)
		m.onlineNovels = refreshed
	} else {
		m.upsertOnlineNovel(novel)
	}
	m.updateSettingItem(SettingRemoveOnlineNovel, func(s *SettingItem) {
		s.IntValue = len(m.onlineNovels)
	})

	if err := m.reloadLibraryFromConfig(); err != nil {
		fmt.Println("Failed to reload library:", err)
		m.upsertHistoryNovel(novel)
		if m.removeMode == SettingRemoveOnlineNovel && m.settingsState == settingsStateRemoving {
			m.refreshRemoveOnlineList(novel.Name)
		}
		return
	}

	progressMap, _ := utils.Load()
	m.refreshHistoryList(progressMap)

	if m.removeMode == SettingRemoveOnlineNovel && m.settingsState == settingsStateRemoving {
		m.refreshRemoveOnlineList(novel.Name)
	}
}

func (m *LibraryModel) reloadLibraryFromConfig() error {
	localNovels, err := library.LoadLocalNovels()
	if err != nil {
		return err
	}

	newLocalByRoot := library.GroupLocalNovelsByRoot(localNovels)

	combined := append([]library.Novel{}, localNovels...)
	if len(m.onlineNovels) > 0 {
		combined = append(combined, m.onlineNovels...)
	}

	progressMap, _ := utils.Load()
	sortNovelsByLastRead(combined, progressMap)

	historyItems := make([]list.Item, len(combined))
	for i := range combined {
		historyItems[i] = combined[i]
	}

	var rootItems []BookshelfItem
	if len(m.onlineNovels) > 0 {
		rootItems = append(rootItems, BookshelfItem{
			Kind:   BookshelfItemDiscovery,
			Name:   lang.Active().Bookshelf.DiscoveryName,
			Path:   library.CacheDir(),
			Novels: m.onlineNovels,
		})
	}
	for _, dir := range utils.AppConfig.Library.Paths {
		cleanDir := filepath.Clean(dir)
		name := filepath.Base(cleanDir)
		if name == "." || name == string(os.PathSeparator) {
			name = cleanDir
		}
		rootItems = append(rootItems, BookshelfItem{
			Kind: BookshelfItemFolder,
			Name: name,
			Path: dir,
		})
	}

	libraryItems := make([]list.Item, len(rootItems))
	for i := range rootItems {
		libraryItems[i] = rootItems[i]
	}

	historyList := m.lists[0]
	historyList.SetItems(historyItems)
	if len(historyItems) > 0 {
		historyList.Select(0)
	}

	libraryList := m.lists[1]
	libraryList.SetItems(libraryItems)
	if len(libraryItems) > 0 {
		libraryList.Select(0)
	}

	m.lists[0] = historyList
	m.lists[1] = libraryList
	m.localByRoot = newLocalByRoot
	m.bookshelfRootItems = rootItems

	return nil
}

func pathsEqual(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// ---------------- Initialization ----------------
func NewLibraryModel() LibraryModel {
	localNovels, err := library.LoadLocalNovels()
	if err != nil {
		fmt.Println("Error loading library:", err)
		os.Exit(1)
	}

	onlineNovels, onlineErr := library.LoadAllCachedNovels()
	if onlineErr != nil && !os.IsNotExist(onlineErr) {
		fmt.Println("Error loading online cache:", onlineErr)
	}

	novels := append(localNovels, onlineNovels...)

	progressMap, _ := utils.Load()
	for i := range novels {
		source := "local"
		if !novels[i].IsLocal {
			source = "online"
		}
		if p, ok := utils.GetProgress(progressMap, novels[i].Name, source); ok {
			if !p.LastRead.IsZero() {
				novels[i].Modified = p.LastRead
			}
			novels[i].Current = p.LastChapter
		}
	}

	sortNovelsByLastRead(novels, progressMap)

	localList := make([]list.Item, len(novels))
	for i, n := range novels {
		localList[i] = n
	}

	localByRoot := library.GroupLocalNovelsByRoot(localNovels)

	sortedOnline := make([]library.Novel, len(onlineNovels))
	copy(sortedOnline, onlineNovels)
	sortOnlineNovels(sortedOnline)

	texts := lang.Active()

	var rootItems []BookshelfItem
	if len(sortedOnline) > 0 {
		rootItems = append(rootItems, BookshelfItem{
			Kind:   BookshelfItemDiscovery,
			Name:   texts.Bookshelf.DiscoveryName,
			Path:   library.CacheDir(),
			Novels: sortedOnline,
		})
	}

	for _, dir := range utils.AppConfig.Library.Paths {
		cleaned := filepath.Clean(dir)
		name := filepath.Base(cleaned)
		if name == "." || name == string(os.PathSeparator) {
			name = cleaned
		}
		rootItems = append(rootItems, BookshelfItem{
			Kind: BookshelfItemFolder,
			Name: name,
			Path: dir,
		})
	}

	historyList := list.New(localList, &NovelDelegate{}, 0, 0)
	listSettings(&historyList)
	filterStyle(&historyList)

	initialLibraryItems := make([]list.Item, len(rootItems))
	for i := range rootItems {
		initialLibraryItems[i] = rootItems[i]
	}
	libraryList := list.New(initialLibraryItems, &NovelDelegate{}, 0, 0)
	listSettings(&libraryList)
	filterStyle(&libraryList)
	if len(initialLibraryItems) > 0 {
		libraryList.Select(0)
	}

	discoveryList := list.New(nil, &NovelDelegate{}, 0, 0)
	listSettings(&discoveryList)
	filterStyle(&discoveryList)

	settingsList := list.New(nil, &NovelDelegate{}, 0, 0)
	listSettings(&settingsList)
	filterStyle(&settingsList)

	removeList := list.New(nil, &NovelDelegate{}, 0, 0)
	listSettings(&removeList)
	filterStyle(&removeList)

	confirmList := list.New(nil, &ConfirmDelegate{}, 0, 0)
	listSettings(&confirmList)
	filterStyle(&confirmList)
	confirmList.SetShowStatusBar(false)
	confirmList.SetShowPagination(false)
	confirmList.SetShowHelp(false)

	ti := textinput.New()
	ti.PromptStyle = gloss.NewStyle().Foreground(gloss.Color("#89b4fa")).PaddingLeft(1).Bold(true)
	ti.PlaceholderStyle = InputPlaceholderStyle
	ti.TextStyle = InputTextStyle
	ti.Cursor.Style = gloss.NewStyle().Foreground(gloss.Color("#89b4fa"))
	ti.Cursor.SetMode(cursor.CursorHide)
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 30

	model := LibraryModel{
		lists:              []list.Model{historyList, libraryList, discoveryList, settingsList},
		removeList:         removeList,
		confirmList:        confirmList,
		activeTab:          0,
		discoveryInput:     ti,
		language:           lang.CurrentLocale(),
		localByRoot:        localByRoot,
		onlineNovels:       sortedOnline,
		bookshelfRootItems: append([]BookshelfItem(nil), rootItems...),
	}

	model.rebuildSettingsList()
	model.applyLanguage()

	return model
}

// ---------------- Sorting utility ----------------
func sortNovelsByLastRead(novels []library.Novel, progress map[string]utils.Progress) {
	sort.SliceStable(novels, func(i, j int) bool {
		ti, okI := lastReadFor(novels[i], progress)
		tj, okJ := lastReadFor(novels[j], progress)
		if okI && okJ {
			return ti.After(tj)
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return novels[i].Modified.After(novels[j].Modified)
	})
}

func lastReadFor(n library.Novel, progress map[string]utils.Progress) (time.Time, bool) {
	source := "local"
	if !n.IsLocal {
		source = "online"
	}
	if p, ok := utils.GetProgress(progress, n.Name, source); ok {
		if !p.LastRead.IsZero() {
			return p.LastRead, true
		}
	}
	return n.Modified, !n.Modified.IsZero()
}

// ---------------- Styles ----------------
var (
	PromptBoxStyle = gloss.NewStyle().
			Border(gloss.RoundedBorder()).
			BorderForeground(gloss.Color("#89b4fa"))

	InputTextStyle = gloss.NewStyle().
			Foreground(gloss.Color("#cdd6f4"))

	InputPlaceholderStyle = gloss.NewStyle().
				Foreground(gloss.Color("#585b70"))

	ConfirmBoxStyle = gloss.NewStyle().
			Border(gloss.RoundedBorder()).
			BorderForeground(gloss.Color("#f38ba8")).
			Padding(1, 2)

	ConfirmPromptStyle = gloss.NewStyle().
				Foreground(gloss.Color("#f38ba8")).
				Bold(true)

	ConfirmListStyle = gloss.NewStyle().
				Foreground(gloss.Color("#cdd6f4"))
)

// ---- Dialog sizing helpers ----
const (
	confirmMaxWidth = 60
	confirmMinWidth = 24
)

func (m *LibraryModel) confirmDialogWidths() (dlgW, contentW int) {
	avail := m.width - 6 // side margin
	if avail < confirmMinWidth {
		avail = confirmMinWidth
	}
	if avail > confirmMaxWidth {
		avail = confirmMaxWidth
	}
	dlgW = avail
	// ConfirmBoxStyle has Padding(1,2) + border → content width = dlgW - 6
	contentW = dlgW - 6
	if contentW < 10 {
		contentW = 10
	}
	return
}

// ---- Compact delegate ONLY for Yes/No ----
type ConfirmDelegate struct{ list.DefaultDelegate }

func (d *ConfirmDelegate) Height() int  { return 1 }
func (d *ConfirmDelegate) Spacing() int { return 0 }
func (d *ConfirmDelegate) Render(w io.Writer, m list.Model, idx int, it list.Item) {
	label := it.(confirmChoice).Label
	if idx == m.Index() {
		fmt.Fprint(w, SelectedTitleStyle.Render(label))
	} else {
		fmt.Fprint(w, NormalTitleStyle.Render(label))
	}
}
