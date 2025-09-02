package ui

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"

	//Start of truncation
	"github.com/mattn/go-runewidth"

	"novel_reader/utils"
)

type Novel struct {
	Name     string
	Path     string
	Latest   string
	Current  string
	Modified time.Time
}

func (n Novel) Title() string       { return n.Name }
func (n Novel) Description() string { return n.Latest }
func (n Novel) FilterValue() string { return n.Name }

// ----- LibraryModel -----
type LibraryModel struct {
	lists     []list.Model
	width     int
	height    int
	tabs      []string
	activeTab int
}

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

	// Centered list with max width
	availWidth := width - 8 // account for padding
	if availWidth > ListMaxWidth {
		availWidth = ListMaxWidth
	}
	m.lists[m.activeTab].SetSize(availWidth, height-5) // 5 lines reserved for tabs
}

func (m LibraryModel) Init() tea.Cmd { return nil }

func (m LibraryModel) Update(msg tea.Msg) (LibraryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.lists[m.activeTab].FilterState() == list.Filtering {
			break
		} else if m.lists[m.activeTab].IsFiltered() {
			m.lists[m.activeTab].ResetFilter()
			break
		}

		switch msg.String() {
		case "esc":
			return m, tea.Quit
		case "tab":
			m.nextTab()
		case "shift+tab":
			m.prevTab()
		}

	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd
	var newList list.Model
	newList, cmd = m.lists[m.activeTab].Update(msg)
	m.lists[m.activeTab] = newList
	return m, cmd
}

func (m LibraryModel) View() string {
	// Tabs
	var renderedTabs []string
	for i, name := range m.tabs {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, ActiveTabStyle.Render(name))
		} else {
			renderedTabs = append(renderedTabs, InactiveTabStyle.Render(name))
		}
	}
	tabsRow := TabsRow.Width(m.width).Render(gloss.JoinHorizontal(gloss.Top, renderedTabs...))

	// Underline
	maxUnderline := 48
	lineWidth := m.width
	if lineWidth > maxUnderline {
		lineWidth = maxUnderline
	}
	underlineRow := UnderlineRow.Width(m.width).Render(strings.Repeat("─", lineWidth))

	// List container
	containerWidth := ListMaxWidth
	if m.width < containerWidth {
		containerWidth = m.width - 8 // leave some margin
	}
	listBlock := ListStyle.Width(containerWidth).Render(m.lists[m.activeTab].View())
	list := List.Width(m.width).Render(listBlock)

	return tabsRow + "\n" + underlineRow + list
}

func LoadLibrary() ([]Novel, error) {
	var novels []Novel
	for _, dir := range utils.AppConfig.Library.Paths {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".txt") {
				info, statErr := os.Stat(path)
				if statErr != nil {
					return statErr
				}

				novels = append(novels, Novel{
					Name:     strings.TrimSuffix(d.Name(), ".txt"),
					Path:     path,
					Latest:   "",             // fill later if needed
					Current:  "",             // fill later if needed
					Modified: info.ModTime(), // placeholder
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// --- Step 2: overwrite Modified with LastRead from saved progress ---
	progressMap, _ := utils.Load()
	for i := range novels {
		if p, ok := progressMap[novels[i].Name]; ok {
			novels[i].Modified = p.LastRead
		}
	}

	// --- optional: sort by most recently read ---
	sort.Slice(novels, func(i, j int) bool {
		return novels[i].Modified.After(novels[j].Modified)
	})

	return novels, nil
}

type NovelDelegate struct {
	list.DefaultDelegate
}

func (d *NovelDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	novel := item.(Novel)

	// Available width for text (subtract 2-4 for padding/borders)
	availWidth := m.Width() - 10

	// Title and description
	title := novel.Title()
	desc := "" + novel.Description() // "" left for when I want to add prefix to it
	desc = runewidth.Truncate(desc, availWidth, "…")

	// Apply styles
	if index == m.Index() {
		title = SelectedTitleStyle.Render(title)
		desc = SelectedDescStyle.Render(desc)
	} else {
		title = NormalTitleStyle.Render(title)
		desc = NormalDescStyle.Render(desc)
	}

	// Render as two lines
	fmt.Fprintf(w, "%s\n%s", title, desc)
}

func (d *NovelDelegate) Height() int {
	return 2 // two lines per item
}

func (d *NovelDelegate) Spacing() int {
	return 1
}

func (d *NovelDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func filterStyle(l *list.Model) {
	l.FilterInput.Prompt = "搜索："
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

func NewLibraryModel() LibraryModel {
	// Load novels
	novels, err := LoadLibrary()
	if err != nil {
		fmt.Println("Error loading library:", err)
		os.Exit(1)
	}

	// Load saved progress
	progressMap, _ := utils.Load()

	// Update each novel's Latest field with last chapter from progress
	for i, n := range novels {
		if p, ok := progressMap[n.Name]; ok {
			novels[i].Latest = p.LastChapter
		}
	}

	// Convert to list items
	historyItems := make([]list.Item, len(novels))
	for i, n := range novels {
		historyItems[i] = n
	}

	historyList := list.New(historyItems, &NovelDelegate{}, 0, 0)
	listSettings(&historyList)
	filterStyle(&historyList)
	
	// placeholders for other tabs
	libraryList := list.New(nil, &NovelDelegate{}, 0, 0)
	listSettings(&libraryList)
	filterStyle(&libraryList)
	discoveryList := list.New(nil, &NovelDelegate{}, 0, 0)
	listSettings(&discoveryList)
	filterStyle(&discoveryList)
	
	libModel := LibraryModel{
	    lists:     []list.Model{historyList, libraryList, discoveryList},
	    tabs:      []string{"游览记录", "书架", "发现"},
	    activeTab: 0,
	}

	return libModel
}

func sortNovelsByLastRead(novels []Novel, progress map[string]utils.Progress) {
	sort.Slice(novels, func(i, j int) bool {
		pi, okI := progress[novels[i].Name]
		pj, okJ := progress[novels[j].Name]

		if okI && okJ {
			return pi.LastRead.After(pj.LastRead)
		} else if okI {
			return true
		} else if okJ {
			return false
		}
		return false
	})
}
