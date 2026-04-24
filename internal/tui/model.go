package tui

import (
	"fmt"
	"sort"
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
	stateBrowse state = iota
	stateLoading
	stateVersionList
	stateReleaseNotes
)

var (
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	officialBadge  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("official")
	partnerBadge   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("partner")
	communityBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("community")
)

func tierBadge(tier string) string {
	switch tier {
	case registry.TierOfficial:
		return officialBadge
	case registry.TierPartner:
		return partnerBadge
	default:
		return communityBadge
	}
}

// tierRank orders official < partner < community for stable secondary sorting.
func tierRank(tier string) int {
	switch tier {
	case registry.TierOfficial:
		return 0
	case registry.TierPartner:
		return 1
	default:
		return 2
	}
}

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

// providersLoadedMsg carries providers fetched for a single tier.
type providersLoadedMsg struct {
	tier      string
	providers []registry.Provider
	err       error
}

// versionItem implements list.Item for displaying a version in the list.
type versionItem struct {
	version   registry.VersionInfo
	published string
}

func (v versionItem) Title() string { return v.version.Version }
func (v versionItem) Description() string {
	if v.published == "" {
		return ""
	}
	return "published: " + v.published
}
func (v versionItem) FilterValue() string { return v.version.Version }

// providerItem implements list.Item for the browse list.
type providerItem struct {
	provider registry.Provider
}

func (p providerItem) Title() string { return p.provider.FullName() }
func (p providerItem) Description() string {
	return fmt.Sprintf("%s  •  %s downloads", tierBadge(p.provider.Tier), humanizeCount(p.provider.Downloads))
}
func (p providerItem) FilterValue() string { return p.provider.FullName() }

func humanizeCount(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// Model is the bubbletea model for the TUI.
type Model struct {
	state state

	// Browse state
	input          textinput.Model
	browseList     list.Model
	allProviders   []registry.Provider
	tiersLoaded    map[string]bool
	tiersLoading   map[string]bool
	loadedCount    int
	browseErr      string
	lastFilterText string

	// Versions / release notes
	spinner       spinner.Model
	versionList   list.Model
	viewport      viewport.Model
	namespace     string
	providerName  string
	source        string
	selectedVer   string
	errorMsg      string
	statusMsg     string
	width, height int

	// Sizing for the version-list / snippet split layout.
	versionListWidth  int
	snippetPaneWidth  int
	snippetPaneHeight int
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "type to filter providers (e.g. azurerm, integrations/github)"
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	versionDelegate := list.NewDefaultDelegate()
	versions := list.New([]list.Item{}, versionDelegate, 0, 0)
	versions.Title = "Versions"
	versions.SetShowStatusBar(false)

	browseDelegate := list.NewDefaultDelegate()
	browse := list.New([]list.Item{}, browseDelegate, 0, 0)
	browse.Title = "Providers"
	browse.SetShowStatusBar(false)
	browse.SetFilteringEnabled(false)
	browse.SetShowHelp(false)

	vp := viewport.New(0, 0)

	return Model{
		state:        stateBrowse,
		input:        ti,
		spinner:      sp,
		browseList:   browse,
		versionList:  versions,
		viewport:     vp,
		tiersLoaded:  map[string]bool{},
		tiersLoading: map[string]bool{registry.TierOfficial: true, registry.TierPartner: true},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		fetchProviders(registry.TierOfficial),
		fetchProviders(registry.TierPartner),
	)
}

// docsURL returns the Terraform Registry docs URL for the given provider version.
func docsURL(namespace, providerName, version string) string {
	return fmt.Sprintf("https://registry.terraform.io/providers/%s/%s/%s/docs", namespace, providerName, version)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		// Reserve room for: header (1) + blank (1) + input (1) + blank (1)
		// + status (1) + help (2). The header is now a single emoji row
		// instead of multi-line ASCII art.
		const headerReserve = 8
		listHeight := msg.Height - headerReserve
		if listHeight < 3 {
			listHeight = 3
		}
		m.browseList.SetSize(msg.Width, listHeight)
		// Other views also render the persistent header, so reserve the
		// same vertical space on top of the per-state chrome.
		//
		// The version list shares the row with a snippet panel showing the
		// `terraform { required_providers { ... } }` block for the selected
		// version. Give the snippet roughly half the width when there is
		// enough room, otherwise let the list use the full width and skip
		// the snippet pane.
		listWidth := msg.Width
		snippetWidth := 0
		const minSnippetWidth = 40
		if msg.Width >= 80 {
			snippetWidth = msg.Width / 2
			if snippetWidth < minSnippetWidth {
				snippetWidth = minSnippetWidth
			}
			listWidth = msg.Width - snippetWidth
		}
		m.versionListWidth = listWidth
		m.snippetPaneWidth = snippetWidth
		m.snippetPaneHeight = listHeight
		m.versionList.SetSize(listWidth, listHeight)
		m.viewport.Width = msg.Width
		m.viewport.Height = listHeight

	case tea.KeyMsg:
		// While the version list is in its built-in filter mode, let the
		// list.Model handle keystrokes itself so that `/`, typing, `esc`
		// (cancel filter), and `enter` (apply filter) work as expected.
		// Otherwise our own back-navigation steals these keys and the user
		// gets stuck inside the filter prompt.
		if m.state == stateVersionList {
			fs := m.versionList.FilterState()
			if fs == list.Filtering || fs == list.FilterApplied {
				// Always let the list handle keys while typing a filter.
				// Once the filter has been applied, only delegate the keys
				// the list itself uses for filter management (esc/`/`) so
				// our `enter`/`d`/`q` shortcuts continue to work on the
				// filtered selection.
				if fs == list.Filtering || msg.String() == "esc" || msg.String() == "/" {
					var cmd tea.Cmd
					m.versionList, cmd = m.versionList.Update(msg)
					return m, cmd
				}
			}
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			switch m.state {
			case stateBrowse:
				// Don't quit on 'q' here so users can type 'q' in the filter.
			case stateReleaseNotes:
				m.state = stateVersionList
				return m, nil
			case stateVersionList:
				m.state = stateBrowse
				m.statusMsg = ""
				m.errorMsg = ""
				m.input.Focus()
				return m, textinput.Blink
			}

		case "esc":
			switch m.state {
			case stateReleaseNotes:
				m.state = stateVersionList
				return m, nil
			case stateVersionList:
				m.state = stateBrowse
				m.statusMsg = ""
				m.errorMsg = ""
				m.input.Focus()
				return m, textinput.Blink
			case stateBrowse:
				if m.input.Value() != "" {
					m.input.SetValue("")
					m.applyFilter()
				}
				return m, nil
			}

		case "d":
			if m.state == stateVersionList {
				if item, ok := m.versionList.SelectedItem().(versionItem); ok {
					url := docsURL(m.namespace, m.providerName, item.version.Version)
					if err := openURL(url); err != nil {
						m.errorMsg = fmt.Sprintf("failed to open browser: %v", err)
						m.statusMsg = ""
					} else {
						m.statusMsg = ""
						m.errorMsg = ""
					}
					return m, nil
				}
			}

		case "enter":
			switch m.state {
			case stateBrowse:
				if item, ok := m.browseList.SelectedItem().(providerItem); ok {
					m.namespace = item.provider.Namespace
					m.providerName = item.provider.Name
					m.state = stateLoading
					m.errorMsg = ""
					return m, tea.Batch(m.spinner.Tick, fetchVersions(m.namespace, m.providerName))
				}
				// Fallback: treat the input as ns/name and look it up directly.
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
				if item, ok := m.versionList.SelectedItem().(versionItem); ok {
					m.selectedVer = item.version.Version
					m.state = stateLoading
					return m, tea.Batch(m.spinner.Tick, fetchReleaseNotes(m.namespace, m.providerName, m.selectedVer))
				}
			}

		case "up", "down", "pgup", "pgdown", "home", "end":
			// In the browse state, route navigation keys to the list so the
			// text input doesn't swallow them.
			if m.state == stateBrowse {
				var cmd tea.Cmd
				m.browseList, cmd = m.browseList.Update(msg)
				return m, cmd
			}
		}

	case providersLoadedMsg:
		m.tiersLoading[msg.tier] = false
		m.tiersLoaded[msg.tier] = true
		if msg.err != nil {
			if m.browseErr == "" {
				m.browseErr = fmt.Sprintf("failed to load %s providers: %v", msg.tier, msg.err)
			}
		} else {
			m.allProviders = append(m.allProviders, msg.providers...)
			m.loadedCount = len(m.allProviders)
			m.sortProviders()
			m.applyFilter()
		}
		return m, nil

	case versionsLoadedMsg:
		if msg.err != nil {
			m.state = stateBrowse
			m.errorMsg = msg.err.Error()
			m.input.Focus()
			return m, textinput.Blink
		}
		m.source = msg.source
		items := make([]list.Item, len(msg.versions))
		for i, v := range msg.versions {
			items[i] = versionItem{version: v, published: v.Published}
		}
		// Clear any filter carried over from a previously-viewed provider
		// before swapping in the new items.
		m.versionList.ResetFilter()
		m.versionList.SetItems(items)
		m.versionList.Select(0)
		m.versionList.Title = fmt.Sprintf("Versions for %s", m.source)
		m.state = stateVersionList
		m.statusMsg = ""
		m.errorMsg = ""
		return m, nil

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
		return m, nil

	case spinner.TickMsg:
		if m.state == stateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		// Also tick while we're still loading provider tiers in the background.
		if m.state == stateBrowse && (m.tiersLoading[registry.TierOfficial] || m.tiersLoading[registry.TierPartner]) {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	// Delegate to the active sub-component.
	switch m.state {
	case stateBrowse:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		// Re-filter when the input value changed.
		if m.input.Value() != m.lastFilterText {
			m.applyFilter()
		}
		return m, cmd
	case stateVersionList:
		var cmd tea.Cmd
		m.versionList, cmd = m.versionList.Update(msg)
		return m, cmd
	case stateReleaseNotes:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// sortProviders orders by tier (official, partner) then downloads desc.
func (m *Model) sortProviders() {
	sort.SliceStable(m.allProviders, func(i, j int) bool {
		ri, rj := tierRank(m.allProviders[i].Tier), tierRank(m.allProviders[j].Tier)
		if ri != rj {
			return ri < rj
		}
		return m.allProviders[i].Downloads > m.allProviders[j].Downloads
	})
}

// applyFilter rebuilds the browse list items from allProviders using the
// current input value as a case-insensitive substring filter on namespace/name.
func (m *Model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.input.Value()))
	m.lastFilterText = m.input.Value()

	items := make([]list.Item, 0, len(m.allProviders))
	for _, p := range m.allProviders {
		if q == "" || strings.Contains(strings.ToLower(p.FullName()), q) {
			items = append(items, providerItem{provider: p})
		}
	}
	m.browseList.SetItems(items)
	if len(items) > 0 {
		m.browseList.Select(0)
	}
}

// renderHelp renders the given help text, word-wrapping it to the current
// terminal width so long key-instruction lines do not spill past the right
// edge of the window. Falls back to a plain render before the first
// WindowSizeMsg has been received (m.width == 0).
func (m Model) renderHelp(s string) string {
	if m.width <= 0 {
		return helpStyle.Render(s)
	}
	return helpStyle.Width(m.width).Render(s)
}

// browseListTitle returns the title for the providers list, including the
// loaded count (and a spinner with pending tiers while still loading).
func (m Model) browseListTitle() string {
	switch {
	case m.tiersLoading[registry.TierOfficial] || m.tiersLoading[registry.TierPartner]:
		var pending []string
		for _, t := range []string{registry.TierOfficial, registry.TierPartner} {
			if m.tiersLoading[t] {
				pending = append(pending, t)
			}
		}
		return fmt.Sprintf("Providers (%d loaded — %s loading %s…)", m.loadedCount, m.spinner.View(), strings.Join(pending, ", "))
	default:
		return fmt.Sprintf("Providers (%d loaded)", m.loadedCount)
	}
}

func (m Model) View() string {
	var b strings.Builder

	// Persistent application header rendered for every state so the user
	// always knows which app they are in. Includes the code-built Terraform
	// logo alongside the title.
	b.WriteString(header(m.width) + "\n\n")

	switch m.state {
	case stateBrowse:
		b.WriteString(m.input.View() + "\n\n")
		if m.browseErr != "" {
			b.WriteString(errorStyle.Render(m.browseErr) + "\n")
		}
		if m.errorMsg != "" {
			b.WriteString(errorStyle.Render("Error: "+m.errorMsg) + "\n")
		}
		m.browseList.Title = m.browseListTitle()
		b.WriteString(m.browseList.View())
		b.WriteString("\n" + m.renderHelp("type to filter • ↑/↓: select • enter: open • esc: clear filter • ctrl+c: quit"))

	case stateLoading:
		b.WriteString(m.spinner.View() + " Fetching data...\n\n")
		b.WriteString(m.renderHelp("ctrl+c: quit"))

	case stateVersionList:
		if m.errorMsg != "" {
			b.WriteString(errorStyle.Render("Error: "+m.errorMsg) + "\n")
		} else if m.statusMsg != "" {
			b.WriteString(statusStyle.Render(m.statusMsg) + "\n")
		}
		b.WriteString(m.versionListWithSnippet())
		b.WriteString("\n" + m.renderHelp("enter: release notes • d: open docs in browser • /: filter • esc: clear filter / back to providers • q: back to providers • required_providers snippet shown on the right"))

	case stateReleaseNotes:
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("Release notes: %s v%s", m.source, m.selectedVer)) + "\n\n")
		b.WriteString(m.viewport.View())
		b.WriteString("\n" + m.renderHelp("↑/↓: scroll • esc/q: back to versions"))
	}

	return b.String()
}

// buildTerraformBlock returns the HCL `terraform { required_providers { ... } }`
// snippet for the given provider and version. The local name in
// `required_providers` uses the provider's short name (e.g. `aws` for
// `hashicorp/aws`), matching the convention shown in the Terraform Registry.
func buildTerraformBlock(namespace, name, version string) string {
	return fmt.Sprintf(`terraform {
  required_providers {
    %s = {
      source  = "%s/%s"
      version = "%s"
    }
  }
}
`, name, namespace, name, version)
}

// renderTerraformSnippet wraps the HCL block in a Markdown ```hcl fence and
// renders it through glamour so it gets syntax highlighting that matches the
// release-notes view. Falls back to the plain HCL on any error.
func renderTerraformSnippet(namespace, name, version string, width int) string {
	body := buildTerraformBlock(namespace, name, version)
	md := "```hcl\n" + body + "```\n"
	if width <= 0 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return body
	}
	out, err := r.Render(md)
	if err != nil {
		return body
	}
	return out
}

// versionListWithSnippet renders the version list with the terraform-block
// snippet for the currently-selected version on the right. When the terminal
// is too narrow to fit a useful snippet pane, only the list is rendered.
func (m Model) versionListWithSnippet() string {
	left := m.versionList.View()
	if m.snippetPaneWidth <= 0 {
		return left
	}
	item, ok := m.versionList.SelectedItem().(versionItem)
	if !ok {
		return left
	}
	right := renderTerraformSnippet(m.namespace, m.providerName, item.version.Version, m.snippetPaneWidth)
	right = lipgloss.NewStyle().
		Width(m.snippetPaneWidth).
		Height(m.snippetPaneHeight).
		Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
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

func fetchProviders(tier string) tea.Cmd {
	return func() tea.Msg {
		providers, err := registry.GetProvidersByTier(tier)
		return providersLoadedMsg{tier: tier, providers: providers, err: err}
	}
}
