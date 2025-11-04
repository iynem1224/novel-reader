package ui

import (
	gloss "github.com/charmbracelet/lipgloss"
)

func ReaderStyle(width int) gloss.Style {
	return gloss.NewStyle().
		Foreground(gloss.Color("#CDD6F4")).
		Width(width).
		PaddingLeft(2).
		PaddingRight(1).
		PaddingTop(1)
}

var ReaderLoadingStyle = gloss.NewStyle().
	Foreground(gloss.Color("#89b4fa")).
	Padding(2).
	Align(gloss.Center)

const (
	TabSpacing    = 4
	TabPaddingTop = 1
	TabPaddingBot = 0
	ListMaxWidth  = 60
)

// Tab styles
var (
	ActiveTabStyle = gloss.NewStyle().
			Foreground(gloss.Color("#89b4fa")).
			Padding(TabPaddingTop, TabSpacing, TabPaddingBot, TabSpacing).
			Align(gloss.Center)

	InactiveTabStyle = gloss.NewStyle().
				Foreground(gloss.Color("#585b70")).
				Padding(TabPaddingTop, TabSpacing, TabPaddingBot, TabSpacing).
				Align(gloss.Center)
)

// List container style
var ListStyle = gloss.NewStyle().
	Align(gloss.Left).
	Padding(1, 4) // left/right padding

// Listed item styles
var (
	SelectedTitleStyle = gloss.NewStyle().
				Foreground(gloss.Color("#89b4fa")).
				BorderLeft(true).
				BorderStyle(gloss.NormalBorder()).
				BorderForeground(gloss.Color("#89b4fa")).
				PaddingLeft(1).
				Bold(true)

	SelectedDescStyle = gloss.NewStyle().
				Foreground(gloss.Color("#bac2de")).
				BorderLeft(true).
				BorderStyle(gloss.NormalBorder()).
				BorderForeground(gloss.Color("#89b4fa")).
				PaddingLeft(1)

	NormalTitleStyle = gloss.NewStyle().
				Foreground(gloss.Color("#585b70")).
				PaddingLeft(2)

	NormalDescStyle = gloss.NewStyle().
			Foreground(gloss.Color("#585b70")).
			PaddingLeft(2)
)

var (
	PromptStyle = gloss.NewStyle().
			Foreground(gloss.Color("#89b4fa"))

	PromptTextStyle = gloss.NewStyle().
			Foreground(gloss.Color("#cdd6f4"))

	PromptCursorStyle = gloss.NewStyle().
				Foreground(gloss.Color("#cdd6f4"))
)

// Tabs
var (
	TabsRow = gloss.NewStyle().
		Foreground(gloss.Color("#89b4fa")).
		Align(gloss.Center).
		Bold(true)

	UnderlineRow = gloss.NewStyle().
			Foreground(gloss.Color("#363a4f")).
			Align(gloss.Center)

	List = gloss.NewStyle().
		Align(gloss.Center)

	StatusStyle = gloss.NewStyle().
			Foreground(gloss.Color("#89b4fa")).
			PaddingLeft(4).
			PaddingRight(4).
			PaddingTop(1).
			Align(gloss.Center)

	StatusMutedStyle = gloss.NewStyle().
				Foreground(gloss.Color("#585b70")).
				PaddingLeft(4).
				PaddingTop(1).
				Align(gloss.Center)
)
