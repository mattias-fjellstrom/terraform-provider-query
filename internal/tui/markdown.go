package tui

import (
	"os"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// detectMarkdownStyle picks a glamour style for the current terminal.
//
// It must be called before the Bubble Tea program starts. Glamour's "auto"
// style queries the terminal for its background color on every render, and
// while Bubble Tea is reading stdin it races with that query: the reply is
// often swallowed, leaving the render blocked until a 5s timeout. Detecting
// once up front (lipgloss caches the result) avoids that entirely.
func detectMarkdownStyle() string {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return styles.NoTTYStyle
	}
	if lipgloss.HasDarkBackground() {
		return styles.DarkStyle
	}
	return styles.LightStyle
}

// renderMarkdown renders md for the terminal using a fixed style, falling back
// to the raw text if rendering fails.
func renderMarkdown(md, style string) string {
	out, err := glamour.Render(md, style)
	if err != nil {
		return md
	}
	return out
}
