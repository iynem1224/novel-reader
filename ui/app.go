package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

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
	m.libraryUI, cmd = m.libraryUI.Update(msg)

	if listMsg, ok := msg.(tea.KeyMsg); ok && listMsg.String() == "enter" {
		selected := m.libraryUI.ActiveList().SelectedItem().(Novel)
		reader := NewReaderModel(selected.Path, selected.Name) // pass Name here

		// Load saved progress
		progressMap, _ := utils.Load()
		if p, ok := progressMap[selected.Name]; ok {
			reader.currentChapter = p.Chapter
			reader.Page = p.Page
		}

		m.readerUI = reader
		m.state = StateReader
		cmd = m.syncWindowSizeCmd()
	}
	return m, cmd
}

func (m AppModel) handleStateReader(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.readerUI, cmd = m.readerUI.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.state = StateLibrary
		
			// reload progress and sort library items
			progressMap, _ := utils.Load()
		
			// choose which list to sort; History tab = 0
			hl := m.libraryUI.ActiveList()
		
			novels := make([]Novel, len(hl.Items()))
			for i, item := range hl.Items() {
				novels[i] = item.(Novel)
			}
		
			sortNovelsByLastRead(novels, progressMap)
		
			items := make([]list.Item, len(novels))
			for i, n := range novels {
				items[i] = n
			}
			hl.SetItems(items)
			hl.Select(0)
		
		case "tab", "t": // open TOC
			m.tocUI = NewTOCModel(
				m.readerUI.TOC,
				m.readerUI.Width,
				m.readerUI.Height,
				m.readerUI.currentChapter, // ← start at current chapter
			)
			m.state = StateTOC
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
		m.readerUI.JumpToChapter(int(msg))
		m.state = StateReader
	case TOCCancelMsg:
		m.state = StateReader
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
		return "Unknown state"
	}
}

func RunApp() {
	// Create the LibraryModel
	library := NewLibraryModel()

	// Wrap it in AppModel
	app := AppModel{
		state:     StateLibrary,
		libraryUI: library,
	}

	// Start Bubble Tea program
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
