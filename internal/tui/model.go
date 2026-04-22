package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/registry"
)

type state int

const (
	stateSearch state = iota
	stateLoading
	stateVersionList
	stateReleaseNotes
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// versionsLoadedMsg carries the loaded version list.
type versionsLoadedMsg struct {
	versions []registry.VersionInfo
	source   string
	err      error
}

// releaseNotesLoadedMsg carries release notes for a version.
type releaseNotesLoadedMsg struct {
	notes string
	err   error
}

// versionItem implements list.Item for displaying a version in the list.
type versionItem struct {
	version   registry.VersionInfo
	published string
}

func (v versionItem) Title() string       { return v.version.Version }
func (v versionItem) Description() string {
	if v.published == "" {
		return ""
	}
	return "published: " + v.published
}
func (v versionItem) FilterValue() string { return v.version.Version }

// Model is the bubbletea model for the TUI.
type Model struct {
	state         state
	input         textinput.Model
	spinner       spinner.Model
	list          list.Model
	viewport      viewport.Model
	namespace     string
	providerName  string
	source        string
	selectedVer   string
	errorMsg      string
	width, height int
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "e.g. azurerm or integrations/github"
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 40

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Versions"
	l.SetShowStatusBar(false)

	vp := viewport.New(0, 0)

	return Model{
		state:   stateSearch,
		input:   ti,
		spinner: sp,
		list:    l,
		viewport: vp,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-6)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 6

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state == stateSearch {
				return m, tea.Quit
			}
			if m.state == stateReleaseNotes {
				m.state = stateVersionList
				return m, nil
			}
			if m.state == stateVersionList {
				m.state = stateSearch
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			}

		case "enter":
			switch m.state {
			case stateSearch:
				input := strings.TrimSpace(m.input.Value())
				if input == "" {
					return m, nil
				}
				namespace, name := registry.ParseProvider(input)
				m.namespace = namespace
				m.providerName = name
				m.state = stateLoading
				m.errorMsg = ""
				return m, tea.Batch(m.spinner.Tick, fetchVersions(namespace, name))

			case stateVersionList:
				if item, ok := m.list.SelectedItem().(versionItem); ok {
					m.selectedVer = item.version.Version
					m.state = stateLoading
					return m, tea.Batch(m.spinner.Tick, fetchReleaseNotes(m.namespace, m.providerName, m.selectedVer))
				}
			}

		case "esc":
			switch m.state {
			case stateReleaseNotes:
				m.state = stateVersionList
				return m, nil
			case stateVersionList:
				m.state = stateSearch
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			}
		}

	case versionsLoadedMsg:
		if msg.err != nil {
			m.state = stateSearch
			m.errorMsg = msg.err.Error()
			m.input.Focus()
			return m, textinput.Blink
		}
		m.source = msg.source
		items := make([]list.Item, len(msg.versions))
		for i, v := range msg.versions {
			items[i] = versionItem{version: v, published: v.Published}
		}
		m.list.SetItems(items)
		m.list.Select(0)
		m.list.Title = fmt.Sprintf("Versions for %s", m.source)
		m.state = stateVersionList

	case releaseNotesLoadedMsg:
		if msg.err != nil {
			m.state = stateVersionList
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		rendered, err := glamour.Render(msg.notes, "auto")
		if err != nil {
			rendered = msg.notes
		}
		m.viewport.SetContent(rendered)
		m.viewport.GotoTop()
		m.state = stateReleaseNotes
		m.errorMsg = ""

	case spinner.TickMsg:
		if m.state == stateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	// Delegate to active sub-component
	switch m.state {
	case stateSearch:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case stateVersionList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case stateReleaseNotes:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	switch m.state {
	case stateSearch:
		b.WriteString(titleStyle.Render("Terraform Provider Query") + "\n\n")
		b.WriteString("Provider name: " + m.input.View() + "\n\n")
		if m.errorMsg != "" {
			b.WriteString(errorStyle.Render("Error: "+m.errorMsg) + "\n\n")
		}
		b.WriteString(helpStyle.Render("enter: search • q/ctrl+c: quit"))

	case stateLoading:
		b.WriteString(titleStyle.Render("Terraform Provider Query") + "\n\n")
		b.WriteString(m.spinner.View() + " Fetching data...\n\n")
		b.WriteString(helpStyle.Render("ctrl+c: quit"))

	case stateVersionList:
		if m.errorMsg != "" {
			b.WriteString(errorStyle.Render("Error: "+m.errorMsg) + "\n")
		}
		b.WriteString(m.list.View())
		b.WriteString("\n" + helpStyle.Render("enter: view release notes • esc/q: back to search"))

	case stateReleaseNotes:
		b.WriteString(titleStyle.Render(fmt.Sprintf("Release notes: %s v%s", m.source, m.selectedVer)) + "\n\n")
		b.WriteString(m.viewport.View())
		b.WriteString("\n" + helpStyle.Render("↑/↓: scroll • esc/q: back to versions"))
	}

	return b.String()
}

func fetchVersions(namespace, providerName string) tea.Cmd {
	return func() tea.Msg {
		versions, source, err := registry.GetVersions(namespace, providerName)
		return versionsLoadedMsg{versions: versions, source: source, err: err}
	}
}

func fetchReleaseNotes(namespace, providerName, version string) tea.Cmd {
	return func() tea.Msg {
		notes, err := registry.GetReleaseNotes(namespace, providerName, version)
		return releaseNotesLoadedMsg{notes: notes, err: err}
	}
}
