package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"

	"novel_reader/lang"
)

// TOCModel wraps a bubbles list to display chapters.
type TOCModel struct {
	list       list.Model
	jumpBuffer string // accumulate number keys
}

type TOCItem struct {
	title string
	index int // actual zero-based chapter index
}

func (i TOCItem) Title() string       { return i.title }
func (i TOCItem) Description() string { return "" }
func (i TOCItem) FilterValue() string { return i.title }

// Messages used to communicate selection/cancel to the parent AppModel
type TOCSelectMsg int
type TOCCancelMsg struct{}

// NewTOCModel builds a TOCModel from a []TOCChapter slice (actual indices).
func NewTOCModel(toc []TOCChapter, width int, height int, selectedActual int) TOCModel {
	items := make([]list.Item, len(toc))
	selectedPos := 0
	for i, ch := range toc {
		items[i] = TOCItem{title: ch.Title, index: ch.Index}
		if ch.Index == selectedActual {
			selectedPos = i
		}
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
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.Styles.StatusBar = gloss.NewStyle().
		Foreground(gloss.Color("#585b70")).
		PaddingBottom(1).
		PaddingLeft(2)
	l.SetShowTitle(false)
	l.SetShowPagination(true)

	applyTOCStrings(&l)

	l.FilterInput.PromptStyle = PromptStyle.PaddingTop(1)
	l.FilterInput.TextStyle = PromptTextStyle
	l.FilterInput.Cursor.Style = PromptCursorStyle

	// remove list’s default padding
	l.Styles.Title = l.Styles.Title.Margin(0).Padding(0)
	l.Styles.FilterPrompt = l.Styles.FilterPrompt.Padding(0)
	l.Styles.FilterCursor = l.Styles.FilterCursor.Padding(0)

	// **set the selected chapter to match current reader position**
	if len(items) > 0 {
		if selectedPos < 0 || selectedPos >= len(items) {
			selectedPos = 0
		}
		l.Select(selectedPos)
	}

	return TOCModel{list: l}
}

func applyTOCStrings(l *list.Model) {
	texts := lang.Active()
	l.Title = texts.TOC.Title
	l.SetStatusBarItemName(texts.TOC.StatusSingular, texts.TOC.StatusPlural)
	l.FilterInput.Prompt = texts.TOC.FilterPrompt
}

func (m *TOCModel) ApplyLanguage() {
	applyTOCStrings(&m.list)
}

func (m TOCModel) Init() tea.Cmd { return nil }

func (m TOCModel) Update(msg tea.Msg) (TOCModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(TOCItem); ok {
				return m, func() tea.Msg { return TOCSelectMsg(item.index) }
			}
		case "esc":
			if m.list.FilterState() == list.Filtering {
				// exit filter input but keep TOC open
				m.list.ResetFilter()
				return m, nil
			}
			if m.list.IsFiltered() {
				// clear applied filter results
				m.list.ResetFilter()
				return m, nil
			}
			// otherwise: exit TOC
			return m, func() tea.Msg { return TOCCancelMsg{} }
		}

		// Only intercept digits if we're NOT filtering
		if m.list.FilterState() != list.Filtering {
			switch keyMsg.String() {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				m.jumpBuffer += keyMsg.String()
				return m, nil
			case "g", "G":
				if m.jumpBuffer != "" {
					var idx int
					fmt.Sscanf(m.jumpBuffer, "%d", &idx)
					idx--
					if idx >= 0 {
						for i, it := range m.list.Items() {
							if tocItem, ok := it.(TOCItem); ok && tocItem.index == idx {
								m.list.Select(i)
								break
							}
						}
					}
					m.jumpBuffer = ""
					return m, nil
				}
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
