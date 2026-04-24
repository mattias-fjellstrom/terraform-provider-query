package tui

import "github.com/charmbracelet/lipgloss"

// terraformPurple is the HashiCorp/Terraform brand purple.
var terraformPurple = lipgloss.Color("#7B42BC")

// headerTitleStyle styles the header text next to the logo.
var headerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(terraformPurple)

// logo is the magnifying-glass emoji used as the app mark, evoking the
// "query/search" nature of the tool.
const logo = "🔍"

// minHeaderWidth is the minimum terminal width at which we render the logo
// alongside the title. Below this we fall back to the title alone to avoid
// wrapping artifacts on very narrow terminals.
const minHeaderWidth = 40

// header renders the persistent application header. When the terminal is
// wide enough, the magnifying-glass logo is drawn to the left of the title;
// on very narrow terminals only the title is shown.
func header(width int) string {
	title := headerTitleStyle.Render(" Terraform Provider Query")
	if width > 0 && width < minHeaderWidth {
		return title
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, logo, title)
}
