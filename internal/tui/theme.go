package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name           string
	TitleFg        lipgloss.Color
	TitleBg        lipgloss.Color
	ControlBarBg   lipgloss.Color
	ControlBarFg   lipgloss.Color
	ActiveBtnFg    lipgloss.Color
	ActiveBtnBg    lipgloss.Color
	InactiveBtnFg  lipgloss.Color
	InactiveBtnBg  lipgloss.Color
	BorderColor    lipgloss.Color
	PeerLabelFg    lipgloss.Color
	StatusFg       lipgloss.Color
}

var midnightTheme = Theme{
	Name:           "midnight",
	TitleFg:        lipgloss.Color("#FAFAFA"),
	TitleBg:        lipgloss.Color("#7D56F4"),
	ControlBarBg:   lipgloss.Color("#1E1E2E"),
	ControlBarFg:   lipgloss.Color("#CDD6F4"),
	ActiveBtnFg:    lipgloss.Color("#1E1E2E"),
	ActiveBtnBg:    lipgloss.Color("#A6E3A1"),
	InactiveBtnFg:  lipgloss.Color("#CDD6F4"),
	InactiveBtnBg:  lipgloss.Color("#F38BA8"),
	BorderColor:    lipgloss.Color("#89B4FA"),
	PeerLabelFg:    lipgloss.Color("#89DCEB"),
	StatusFg:       lipgloss.Color("#F9E2AF"),
}

var daylightTheme = Theme{
	Name:           "daylight",
	TitleFg:        lipgloss.Color("#111111"),
	TitleBg:        lipgloss.Color("#00BFFF"),
	ControlBarBg:   lipgloss.Color("#E0E0E0"),
	ControlBarFg:   lipgloss.Color("#333333"),
	ActiveBtnFg:    lipgloss.Color("#FFFFFF"),
	ActiveBtnBg:    lipgloss.Color("#2E8B57"),
	InactiveBtnFg:  lipgloss.Color("#FFFFFF"),
	InactiveBtnBg:  lipgloss.Color("#DC143C"),
	BorderColor:    lipgloss.Color("#4682B4"),
	PeerLabelFg:    lipgloss.Color("#000080"),
	StatusFg:       lipgloss.Color("#DAA520"),
}

var ayuTheme = Theme{
	Name:           "ayu",
	TitleFg:        lipgloss.Color("#E6E1CF"),
	TitleBg:        lipgloss.Color("#FFB454"),
	ControlBarBg:   lipgloss.Color("#14191F"),
	ControlBarFg:   lipgloss.Color("#E6E1CF"),
	ActiveBtnFg:    lipgloss.Color("#0F1419"),
	ActiveBtnBg:    lipgloss.Color("#91B362"),
	InactiveBtnFg:  lipgloss.Color("#E6E1CF"),
	InactiveBtnBg:  lipgloss.Color("#F07178"),
	BorderColor:    lipgloss.Color("#3E4B59"),
	PeerLabelFg:    lipgloss.Color("#39BAE6"),
	StatusFg:       lipgloss.Color("#E6B673"),
}

var themes = []Theme{midnightTheme, daylightTheme, ayuTheme}
var currentThemeIdx = 0

// GetCurrentTheme returns the currently active theme
func GetCurrentTheme() Theme {
	return themes[currentThemeIdx]
}

// NextTheme cycles to the next theme and returns it
func NextTheme() Theme {
	currentThemeIdx = (currentThemeIdx + 1) % len(themes)
	return themes[currentThemeIdx]
}
