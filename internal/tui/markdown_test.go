package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestDetectMarkdownStyleNeverAuto(t *testing.T) {
	// "auto" re-queries the terminal on every render, which hangs while
	// Bubble Tea owns stdin.
	if got := detectMarkdownStyle(); got == styles.AutoStyle {
		t.Fatalf("detectMarkdownStyle() = %q; must resolve to a concrete style", got)
	}
}

func TestNewResolvesMarkdownStyle(t *testing.T) {
	if got := New().markdownStyle; got == "" || got == styles.AutoStyle {
		t.Fatalf("New().markdownStyle = %q; want a concrete style", got)
	}
}

func TestRenderMarkdown(t *testing.T) {
	for _, style := range []string{styles.NoTTYStyle, styles.DarkStyle, styles.LightStyle} {
		out := ansi.Strip(renderMarkdown("## Bug fixes\n\n* fixed a thing", style))
		if !strings.Contains(out, "fixed a thing") {
			t.Errorf("style %q: rendered output missing content: %q", style, out)
		}
	}
}
