package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
)

// TOCModel wraps a bubbles list to display chapters.
type TOCModel struct {
	list       list.Model
	jumpBuffer string // accumulate number keys
}

type TOCItem struct {
	title string
	index int
}

func (i TOCItem) Title() string       { return i.title }
func (i TOCItem) Description() string { return "" }
func (i TOCItem) FilterValue() string { return i.title }

// Messages used to communicate selection/cancel to the parent AppModel
type TOCSelectMsg int
type TOCCancelMsg struct{}

// NewTOCModel builds a TOCModel from your []Chapter slice.
// width is used to set the list width initially; we'll resize on WindowSizeMsg.
func NewTOCModel(toc []Chapter, width int, height int, selectedIndex int) TOCModel {
	items := make([]list.Item, len(toc))
	for i, ch := range toc {
		items[i] = TOCItem{title: ch.Title, index: i}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(gloss.Color("229")).Bold(true)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.PaddingLeft(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.PaddingLeft(0)
	delegate.Styles.SelectedTitle = SelectedTitleStyle
	delegate.Styles.NormalTitle = NormalTitleStyle

	delegate.ShowDescription = false

	l := list.New(items, delegate, width, height)
	l.Title = "Table of Contents"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetStatusBarItemName("章", "章")
	l.Styles.StatusBar = gloss.NewStyle().
		Foreground(gloss.Color("#585b70")).
		PaddingBottom(1).
		PaddingLeft(2)
	l.SetShowTitle(false)
	l.SetShowPagination(true)

	l.FilterInput.Prompt = "搜索："
	l.FilterInput.PromptStyle = PromptStyle.PaddingTop(1)
	l.FilterInput.TextStyle = PromptTextStyle
	l.FilterInput.Cursor.Style = PromptCursorStyle

	// remove list’s default padding
	l.Styles.Title = l.Styles.Title.Margin(0).Padding(0)
	l.Styles.FilterPrompt = l.Styles.FilterPrompt.Padding(0)
	l.Styles.FilterCursor = l.Styles.FilterCursor.Padding(0)

	// **set the selected chapter to match current reader position**
	if selectedIndex >= 0 && selectedIndex < len(items) {
		l.Select(selectedIndex)
	}

	return TOCModel{list: l}
}

func (m TOCModel) Init() tea.Cmd { return nil }

func (m TOCModel) Update(msg tea.Msg) (TOCModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(TOCItem); ok {
				return m, func() tea.Msg { return TOCSelectMsg(item.index) }
			}
		case "esc", "q":
			return m, func() tea.Msg { return TOCCancelMsg{} }

		// Capture digits before list sees them
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.jumpBuffer += keyMsg.String()
			return m, nil

		// Our custom “number+G” motion
		case "g", "G":
			if m.jumpBuffer != "" {
				var idx int
				fmt.Sscanf(m.jumpBuffer, "%d", &idx)
				idx-- // convert 1-based → 0-based
				if idx >= 0 && idx < len(m.list.Items()) {
					m.list.Select(idx)
				}
				m.jumpBuffer = ""
				return m, nil
			}
		}
	}

	// If not handled, let list process the key normally
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m TOCModel) View() string {
	// apply padding around the whole list
	return gloss.NewStyle().
		PaddingTop(1).
		PaddingLeft(2).
		Render(m.list.View())
}
