package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

func renderControls(termW int, camOn, micOn bool, theme Theme) string {
	btnStyle := lipgloss.NewStyle().Padding(0, 1).MarginRight(1).Bold(true)

	renderBtn := func(label string, active bool) string {
		if active {
			return btnStyle.Foreground(theme.ActiveBtnFg).Background(theme.ActiveBtnBg).Render(fmt.Sprintf("%s ON", label))
		}
		return btnStyle.Foreground(theme.InactiveBtnFg).Background(theme.InactiveBtnBg).Render(fmt.Sprintf("%s OFF", label))
	}

	vidBtn := renderBtn("[V] Video", camOn)
	micBtn := renderBtn("[M] Mic", micOn)
	themeBtn := btnStyle.Foreground(theme.ControlBarBg).Background(theme.ControlBarFg).Render("[T] Theme")
	quitBtn := btnStyle.Foreground(theme.TitleFg).Background(theme.TitleBg).Render("[Q] Quit")

	content := lipgloss.JoinHorizontal(lipgloss.Center, vidBtn, micBtn, themeBtn, quitBtn)

	return lipgloss.NewStyle().
		Width(termW).
		Align(lipgloss.Center).
		Padding(1, 0).
		Background(theme.ControlBarBg).
		Render(content)
}
