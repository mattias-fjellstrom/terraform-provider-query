package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// terraformPurple is the HashiCorp/Terraform brand purple.
var terraformPurple = lipgloss.Color("#7B42BC")

// logoStyle paints the Terraform mark in the brand purple.
var logoStyle = lipgloss.NewStyle().Foreground(terraformPurple).Bold(true)

// headerTitleStyle styles the header text next to the logo.
var headerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(terraformPurple)

// terraformLogo renders the iconic Terraform "stacked T" mark using Unicode
// half-block characters. Built entirely in code — no image assets.
//
// The mark is two angled "T" shapes stacked vertically:
//
//	▟█  █▙
//	 ▜█  █▛
//	▟█  █▙
//	 ▜█  █▛
func terraformLogo() string {
	lines := []string{
		"▟█  █▙",
		" ▜█  █▛",
		"▟█  █▙",
		" ▜█  █▛",
	}
	return logoStyle.Render(strings.Join(lines, "\n"))
}

// minHeaderWidth is the minimum terminal width at which we render the logo
// alongside the title. Below this we fall back to the title alone to avoid
// wrapping artifacts on very narrow terminals.
const minHeaderWidth = 40

// header renders the persistent application header. When the terminal is
// wide enough, the Terraform logo is drawn to the left of the title; on very
// narrow terminals only the title is shown.
func header(width int) string {
	title := headerTitleStyle.Render(" Terraform Provider Query")
	if width > 0 && width < minHeaderWidth {
		return title
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, terraformLogo(), title)
}
