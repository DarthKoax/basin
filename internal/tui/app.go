package tui

import (
	"fmt"
	"strings"

	"peanutclone/internal/config"
	"peanutclone/internal/domain"
	"peanutclone/internal/resolver"
	"peanutclone/internal/search"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glamourStyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

type tuiMode int

const (
	FinderMode tuiMode = iota
	BrowserMode
	TagsMode
	WizardMode
)

// Msg types
type suggestionMsg struct {
	suggestions []string
	err         error
}

type browserItem struct {
	name   string
	isDir  bool
	isFile bool
	isSnip bool
	snip   domain.Snippet
	path   string // Virtual path structure (e.g. "nested/docker")
}

// AppModel holds the state for the Bubble Tea application.
type AppModel struct {
	termWidth    int // terminal width in characters
	termHeight   int // terminal height in characters
	mode         tuiMode
	config       *config.Config
	allSnippets  []domain.Snippet
	snippetFiles []domain.SnippetFile
	err          error

	// Shared exits
	Done         bool
	FinalSnippet string

	// Finder Mode state
	finderQuery         textinput.Model
	finderSelectedIndex int
	finderScrollOffset  int
	finderFocusedPanel  int // 0=list, 1=preview

	// Browser Mode state
	browserPath          string
	browserItems         []browserItem
	browserSelectedIndex int
	browserScrollOffset  int

	// Tags Mode state
	tags                     []string
	tagCounts                map[string]int
	tagsSelectedIndex        int
	tagsScrollOffset         int
	selectedTag              string
	tagSnippets              []domain.Snippet
	tagSnippetsSelectedIndex int
	tagSnippetsScrollOffset  int
	inTagSnippets            bool

	// Wizard Mode state
	wizardSnippet          domain.Snippet
	wizardConfirmed        map[string]string
	wizardVarIndex         int
	wizardInput            textinput.Model
	wizardSuggestions      []string
	wizardSugSelectedIndex int
	wizardSugScrollOffset  int
	wizardLoadingSuggs     bool
	wizardError            error

	// Preview scroll offsets
	finderPreviewScroll int
	wizardPreviewScroll int
}

// NewAppModel initializes the main Bubble Tea application model.
func NewAppModel(cfg *config.Config, snippets []domain.Snippet, files []domain.SnippetFile) *AppModel {
	ti := textinput.New()
	ti.Placeholder = "Type to search snippets..."
	ti.Focus()

	wInput := textinput.New()
	wInput.Focus()

	m := &AppModel{
		mode:            FinderMode,
		config:          cfg,
		allSnippets:     snippets,
		snippetFiles:    files,
		finderQuery:     ti,
		wizardInput:     wInput,
		wizardConfirmed: make(map[string]string),
	}

	m.initBrowser()
	m.initTags()

	return m
}

func (m *AppModel) Init() tea.Cmd {
	return textinput.Blink
}

// Sub-state builders

func (m *AppModel) initBrowser() {
	m.browserItems = m.getBrowserItemsForPath(m.browserPath)
	m.browserSelectedIndex = 0
}

func (m *AppModel) getBrowserItemsForPath(dir string) []browserItem {
	var items []browserItem
	seen := make(map[string]bool)

	// Normalized comparison directory
	dir = strings.Trim(dir, "/")

	for _, file := range m.snippetFiles {
		rel := strings.Trim(file.RelativePath, "/")

		// Check if file is in this dir
		if dir == "" {
			// Root level
			idx := strings.Index(rel, "/")
			if idx == -1 {
				// File at root
				items = append(items, browserItem{name: rel, isFile: true, path: rel})
			} else {
				// Directory prefix
				dirPrefix := rel[:idx]
				if !seen[dirPrefix] {
					seen[dirPrefix] = true
					items = append(items, browserItem{name: dirPrefix, isDir: true, path: dirPrefix})
				}
			}
		} else {
			// Sub-directory level
			if strings.HasPrefix(rel, dir+"/") {
				sub := strings.TrimPrefix(rel, dir+"/")
				idx := strings.Index(sub, "/")
				if idx == -1 {
					// File in this directory
					items = append(items, browserItem{name: sub, isFile: true, path: dir + "/" + sub})
				} else {
					dirPrefix := sub[:idx]
					fullDir := dir + "/" + dirPrefix
					if !seen[fullDir] {
						seen[fullDir] = true
						items = append(items, browserItem{name: dirPrefix, isDir: true, path: fullDir})
					}
				}
			}
		}
	}

	// If the current path points to a file, list the snippets inside that file!
	for _, file := range m.snippetFiles {
		rel := strings.Trim(file.RelativePath, "/")
		if rel == dir {
			for _, snip := range file.Snippets {
				items = append(items, browserItem{name: snip.Name, isSnip: true, snip: snip, path: dir})
			}
		}
	}

	return items
}

func (m *AppModel) initTags() {
	m.tagCounts = make(map[string]int)
	var tagsList []string
	seen := make(map[string]bool)

	untaggedCount := 0

	for _, snip := range m.allSnippets {
		if len(snip.Tags) == 0 {
			untaggedCount++
			continue
		}
		for _, tag := range snip.Tags {
			m.tagCounts[tag]++
			if !seen[tag] {
				seen[tag] = true
				tagsList = append(tagsList, tag)
			}
		}
	}

	m.tags = tagsList
	if untaggedCount > 0 {
		m.tagCounts["(untagged)"] = untaggedCount
		m.tags = append(m.tags, "(untagged)")
	}
	m.tagsSelectedIndex = 0
}

func (m *AppModel) filterSnippetsByTag(tag string) {
	var filtered []domain.Snippet
	for _, snip := range m.allSnippets {
		if tag == "(untagged)" {
			if len(snip.Tags) == 0 {
				filtered = append(filtered, snip)
			}
		} else {
			for _, t := range snip.Tags {
				if t == tag {
					filtered = append(filtered, snip)
					break
				}
			}
		}
	}
	m.tagSnippets = filtered
	m.tagSnippetsSelectedIndex = 0
}

// Transition to wizard or output

func (m *AppModel) selectSnippet(snip domain.Snippet) tea.Cmd {
	if len(snip.Placeholders) == 0 {
		// Zero-variable snippet, finalise immediately
		return m.finalizeSnippet(snip.Body)
	}

	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	return m.setupWizardForCurrentVar()
}

func (m *AppModel) setupWizardForCurrentVar() tea.Cmd {
	if m.wizardVarIndex >= len(m.wizardSnippet.Placeholders) {
		r := resolver.NewResolver(m.config, m.wizardSnippet, domain.Frontmatter{})
		rendered := r.RenderSnippet(m.wizardConfirmed)
		return m.finalizeSnippet(rendered)
	}

	p := m.wizardSnippet.Placeholders[m.wizardVarIndex]
	r := resolver.NewResolver(m.config, m.wizardSnippet, domain.Frontmatter{})

	// Pre-fill default value
	defVal := r.GetDefaultValue(p)
	m.wizardInput.SetValue(defVal)
	m.wizardInput.Placeholder = fmt.Sprintf("Value for %s...", p.Name)

	m.wizardSuggestions = nil
	m.wizardSugSelectedIndex = 0
	m.wizardError = nil

	// Check if suggestions are command-backed
	isCmd := p.IsCommand

	if !isCmd {
		// Lookup in frontmatter or config
		// (Frontmatter of source file)
		var sourceFM domain.Frontmatter
		for _, file := range m.snippetFiles {
			if file.Path == m.wizardSnippet.SourceFile {
				sourceFM = file.Frontmatter
				break
			}
		}
		if fmVar, ok := sourceFM.Variables[p.Name]; ok && fmVar.Command != "" {
			isCmd = true
		} else if cfgVar, ok := m.config.Variables[p.Name]; ok && cfgVar.Command != "" {
			isCmd = true
		}
	}

	if isCmd {
		m.wizardLoadingSuggs = true
		// Run command in background
		return func() tea.Msg {
			// Find frontmatter
			var sourceFM domain.Frontmatter
			for _, file := range m.snippetFiles {
				if file.Path == m.wizardSnippet.SourceFile {
					sourceFM = file.Frontmatter
					break
				}
			}
			res := resolver.NewResolver(m.config, m.wizardSnippet, sourceFM)
			suggs, err := res.GetSuggestions(p, m.wizardConfirmed)
			return suggestionMsg{suggestions: suggs, err: err}
		}
	}

	// Static suggestions or builtins
	var sourceFM domain.Frontmatter
	for _, file := range m.snippetFiles {
		if file.Path == m.wizardSnippet.SourceFile {
			sourceFM = file.Frontmatter
			break
		}
	}
	res := resolver.NewResolver(m.config, m.wizardSnippet, sourceFM)
	suggs, _ := res.GetSuggestions(p, m.wizardConfirmed)
	m.wizardSuggestions = suggs
	m.wizardLoadingSuggs = false
	return nil
}

func (m *AppModel) finalizeSnippet(body string) tea.Cmd {
	m.Done = true
	m.FinalSnippet = body
	_ = clipboard.WriteAll(body)
	return tea.Quit
}

// Main Update Loop

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+d":
			return m, tea.Quit
		case "ctrl+t":
			if m.mode != WizardMode {
				m.mode = (m.mode + 1) % 3
				m.finderFocusedPanel = 0
				m.finderQuery.Focus()
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		return m, nil
	case suggestionMsg:
		m.wizardLoadingSuggs = false
		if msg.err != nil {
			m.wizardError = msg.err
		} else {
			m.wizardSuggestions = msg.suggestions
		}
		return m, nil
	}

	// Delegate based on mode
	switch m.mode {
	case FinderMode:
		return m.updateFinder(msg)
	case BrowserMode:
		return m.updateBrowser(msg)
	case TagsMode:
		return m.updateTags(msg)
	case WizardMode:
		return m.updateWizard(msg)
	}

	return m, nil
}

func (m *AppModel) updateFinder(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
	_, _, panelHeight := m.panelDims()
	visibleH := m.innerContentHeight(panelHeight)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()

		if m.finderFocusedPanel == 1 {
			switch keyStr {
			case "up", "ctrl+k":
				m.finderPreviewScroll--
				if m.finderPreviewScroll < 0 {
					m.finderPreviewScroll = 0
				}
			case "down", "ctrl+j":
				m.finderPreviewScroll++
			case "ctrl+u":
				m.finderPreviewScroll -= visibleH / 2
				if m.finderPreviewScroll < 0 {
					m.finderPreviewScroll = 0
				}
			case "left", "esc", "tab":
				m.finderFocusedPanel = 0
				m.finderQuery.Focus()
			case "enter", "ctrl+c":
				if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
					return m, m.selectSnippet(filtered[m.finderSelectedIndex])
				}
			}
			return m, nil
		}

		switch keyStr {
		case "up", "ctrl+k":
			if m.finderSelectedIndex > 0 {
				m.finderSelectedIndex--
				m.finderPreviewScroll = 0
			}
		case "down", "ctrl+j":
			if m.finderSelectedIndex < len(filtered)-1 {
				m.finderSelectedIndex++
				m.finderPreviewScroll = 0
			}
		case "right", "tab":
			m.finderFocusedPanel = 1
			m.finderQuery.Blur()
		case "enter", "ctrl+c":
			if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
				return m, m.selectSnippet(filtered[m.finderSelectedIndex])
			}
		default:
			m.finderQuery, cmd = m.finderQuery.Update(msg)
			m.finderSelectedIndex = 0
			m.finderScrollOffset = 0
			m.finderPreviewScroll = 0
		}
	}

	m.finderScrollOffset = clampScroll(m.finderSelectedIndex, m.finderScrollOffset, visibleH)

	return m, cmd
}

func (m *AppModel) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	availH := m.termHeight - 2
	if availH < 1 {
		availH = 1
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		switch keyStr {
		case "up", "ctrl+k":
			if m.browserSelectedIndex > 0 {
				m.browserSelectedIndex--
			}
		case "down", "ctrl+j":
			if m.browserSelectedIndex < len(m.browserItems)-1 {
				m.browserSelectedIndex++
			}
		case "enter":
			if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
				item := m.browserItems[m.browserSelectedIndex]
				if item.isDir || item.isFile {
					m.browserPath = item.path
					m.initBrowser()
					m.browserScrollOffset = 0
				} else if item.isSnip {
					return m, m.selectSnippet(item.snip)
				}
			}
		case "esc", "backspace":
			if m.browserPath != "" {
				idx := strings.LastIndex(m.browserPath, "/")
				if idx == -1 {
					m.browserPath = ""
				} else {
					m.browserPath = m.browserPath[:idx]
				}
				m.initBrowser()
				m.browserScrollOffset = 0
			}
		}
	}

	m.browserScrollOffset = clampScroll(m.browserSelectedIndex, m.browserScrollOffset, availH)

	return m, nil
}

func (m *AppModel) updateTags(msg tea.Msg) (tea.Model, tea.Cmd) {
	availH := m.termHeight - 2
	if availH < 1 {
		availH = 1
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		if m.inTagSnippets {
			switch keyStr {
			case "up", "ctrl+k":
				if m.tagSnippetsSelectedIndex > 0 {
					m.tagSnippetsSelectedIndex--
				}
			case "down", "ctrl+j":
				if m.tagSnippetsSelectedIndex < len(m.tagSnippets)-1 {
					m.tagSnippetsSelectedIndex++
				}
			case "enter":
				if len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
					return m, m.selectSnippet(m.tagSnippets[m.tagSnippetsSelectedIndex])
				}
			case "esc", "backspace":
				m.inTagSnippets = false
				m.tagSnippetsScrollOffset = 0
			}
			m.tagSnippetsScrollOffset = clampScroll(m.tagSnippetsSelectedIndex, m.tagSnippetsScrollOffset, availH)
		} else {
			switch keyStr {
			case "up", "ctrl+k":
				if m.tagsSelectedIndex > 0 {
					m.tagsSelectedIndex--
				}
			case "down", "ctrl+j":
				if m.tagsSelectedIndex < len(m.tags)-1 {
					m.tagsSelectedIndex++
				}
			case "enter":
				if len(m.tags) > 0 && m.tagsSelectedIndex < len(m.tags) {
					m.selectedTag = m.tags[m.tagsSelectedIndex]
					m.filterSnippetsByTag(m.selectedTag)
					m.inTagSnippets = true
					m.tagSnippetsScrollOffset = 0
				}
			}
			m.tagsScrollOffset = clampScroll(m.tagsSelectedIndex, m.tagsScrollOffset, availH)
		}
	}
	return m, nil
}

func (m *AppModel) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	_, _, panelHeight := m.panelDims()
	visibleH := m.innerContentHeight(panelHeight)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		switch keyStr {
		case "up", "ctrl+p":
			if len(m.wizardSuggestions) > 0 && m.wizardSugSelectedIndex > 0 {
				m.wizardSugSelectedIndex--
				m.wizardInput.SetValue(m.wizardSuggestions[m.wizardSugSelectedIndex])
			}
		case "down", "ctrl+n":
			if len(m.wizardSuggestions) > 0 && m.wizardSugSelectedIndex < len(m.wizardSuggestions)-1 {
				m.wizardSugSelectedIndex++
				m.wizardInput.SetValue(m.wizardSuggestions[m.wizardSugSelectedIndex])
			}
		case "ctrl+u":
			m.wizardPreviewScroll -= visibleH / 2
			if m.wizardPreviewScroll < 0 {
				m.wizardPreviewScroll = 0
			}
		case "ctrl+d":
			m.wizardPreviewScroll += visibleH / 2
		case "enter", "tab":
			p := m.wizardSnippet.Placeholders[m.wizardVarIndex]
			val := m.wizardInput.Value()
			m.wizardConfirmed[p.Name] = val

			m.wizardVarIndex++
			m.wizardPreviewScroll = 0
			return m, m.setupWizardForCurrentVar()
		case "shift+tab":
			if m.wizardVarIndex > 0 {
				m.wizardVarIndex--
				pPrev := m.wizardSnippet.Placeholders[m.wizardVarIndex]
				delete(m.wizardConfirmed, pPrev.Name)
				m.wizardPreviewScroll = 0
				return m, m.setupWizardForCurrentVar()
			}
		case "alt+enter", "ctrl+j":
			m.wizardInput.SetValue(m.wizardInput.Value() + "\n")
		case "esc":
			m.mode = FinderMode
			m.wizardPreviewScroll = 0
			m.finderFocusedPanel = 0
			m.finderQuery.Focus()
			return m, nil
		default:
			m.wizardInput, cmd = m.wizardInput.Update(msg)
		}
	}

	sugVisibleH := visibleH / 3
	if sugVisibleH < 3 {
		sugVisibleH = 3
	}
	m.wizardSugScrollOffset = clampScroll(m.wizardSugSelectedIndex, m.wizardSugScrollOffset, sugVisibleH)

	return m, cmd
}

// Rendering Views

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5F5FAF")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#AFAF87"))

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#5FD7FF"))

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EEEEEE"))

	previewBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(1)

	focusedBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#5F5FAF")).
			Padding(1)

	instructionsStyle = lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("#888888"))

	mdRenderer, _ = glamour.NewTermRenderer(
		glamour.WithStyles(glamourStyles.DarkStyleConfig),
		glamour.WithWordWrap(0),
	)
)

// panelDims returns the dimensions for the left and right panels, reserving
// room for the header region (title + instructions + search input = 3 rows).
func (m *AppModel) panelDims() (leftWidth, rightWidth, height int) {
	const headerRows = 3
	availHeight := m.termHeight - headerRows
	if availHeight < 1 {
		availHeight = 10
	}
	height = availHeight * m.config.Panel.HeightPercent / 105
	if height < 1 {
		height = availHeight
	}

	leftWidth = m.termWidth * m.config.Panel.WidthPercent / 105
	if leftWidth < 1 {
		leftWidth = 40
	}
	rightWidth = m.termWidth - leftWidth - 5
	if rightWidth < 1 {
		rightWidth = 20
	}
	return
}

func (m *AppModel) innerContentHeight(panelHeight int) int {
	h := panelHeight - 4
	if h < 1 {
		h = 1
	}
	return h
}

func clampScroll(selected, offset, visibleHeight int) int {
	if visibleHeight <= 0 {
		return 0
	}
	if selected < offset {
		return selected
	}
	if selected >= offset+visibleHeight {
		return selected - visibleHeight + 1
	}
	return offset
}

func clipLines(lines []string, offset, maxLines int) []string {
	if maxLines <= 0 || len(lines) == 0 {
		return nil
	}
	if offset >= len(lines) {
		offset = len(lines) - 1
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end]
}

func (m *AppModel) View() string {
	if m.Done {
		return fmt.Sprintf("Snippet copied to clipboard!\n\n%s\n", m.FinalSnippet)
	}

	var sb strings.Builder

	// Render header title
	sb.WriteString(titleStyle.Render("PEANUTCLONE") + " ")

	switch m.mode {
	case FinderMode:
		if m.finderFocusedPanel == 0 {
			sb.WriteString(instructionsStyle.Render("[Ctrl+T: mode]  Tab/Enter: focus preview  Ctrl+C: wizard  Ctrl+D: quit") + "\n")
		} else {
			sb.WriteString(instructionsStyle.Render("[Ctrl+T: mode]  Tab/Esc/Left: back to list  Ctrl+C: wizard  Ctrl+D: quit  ↑↓: scroll") + "\n")
		}
		sb.WriteString(m.finderQuery.View() + "\n")
		sb.WriteString(m.renderMainPanels())
	case BrowserMode:
		sb.WriteString(instructionsStyle.Render(fmt.Sprintf("[Browser: /%s]  Ctrl+T: mode  Enter: open/pick  Esc: up", m.browserPath)) + "\n\n")

		var listLines []string
		for i, item := range m.browserItems {
			prefix := "  "
			if i == m.browserSelectedIndex {
				prefix = "▸ "
			}

			label := item.name
			if item.isDir {
				label = "📁 " + label + "/"
			} else if item.isFile {
				label = "📄 " + label
			} else if item.isSnip {
				label = "⚡ " + label
			}

			if i == m.browserSelectedIndex {
				listLines = append(listLines, selectedItemStyle.Render(prefix+label))
			} else {
				listLines = append(listLines, normalItemStyle.Render(prefix+label))
			}
		}

		availH := m.termHeight - 2
		if availH < 1 {
			availH = 1
		}
		visibleLines := clipLines(listLines, m.browserScrollOffset, availH)
		listContent := strings.Join(visibleLines, "\n")
		if listContent == "" {
			listContent = "  Empty directory."
		}
		sb.WriteString(listContent)
	case TagsMode:
		availH := m.termHeight - 2
		if availH < 1 {
			availH = 1
		}

		if m.inTagSnippets {
			sb.WriteString(instructionsStyle.Render(fmt.Sprintf("[Tag: %s]  Enter: pick  Esc: back", m.selectedTag)) + "\n\n")

			var listLines []string
			for i, snip := range m.tagSnippets {
				label := fmt.Sprintf("  %s (%s)", snip.Name, snip.RelativePath)
				if i == m.tagSnippetsSelectedIndex {
					listLines = append(listLines, selectedItemStyle.Render("▸"+label[1:]))
				} else {
					listLines = append(listLines, normalItemStyle.Render(label))
				}
			}
			visibleLines := clipLines(listLines, m.tagSnippetsScrollOffset, availH)
			listContent := strings.Join(visibleLines, "\n")
			sb.WriteString(listContent)
		} else {
			sb.WriteString(instructionsStyle.Render("[Tags Dashboard]  Ctrl+T: mode  Enter: open") + "\n\n")

			var listLines []string
			for i, tag := range m.tags {
				count := m.tagCounts[tag]
				label := fmt.Sprintf("  #%s (%d)", tag, count)
				if i == m.tagsSelectedIndex {
					listLines = append(listLines, selectedItemStyle.Render("▸"+label[1:]))
				} else {
					listLines = append(listLines, normalItemStyle.Render(label))
				}
			}
			visibleLines := clipLines(listLines, m.tagsScrollOffset, availH)
			listContent := strings.Join(visibleLines, "\n")
			sb.WriteString(listContent)
		}
	case WizardMode:
		sb.WriteString(instructionsStyle.Render("[Ctrl+T: mode]  Enter: next var  Shift+Tab: back  Esc: cancel") + "\n")
		sb.WriteString(m.finderQuery.View() + "\n")

		wizardContent := m.renderWizardContent()

		wizardWidth, previewWidth, panelHeight := m.panelDims()
		innerH := m.innerContentHeight(panelHeight)

		rendered := m.renderWizardPreview()
		previewLines := strings.Split(rendered, "\n")

		maxPreviewScroll := len(previewLines) - innerH
		if maxPreviewScroll < 0 {
			maxPreviewScroll = 0
		}
		if m.wizardPreviewScroll > maxPreviewScroll {
			m.wizardPreviewScroll = maxPreviewScroll
		}
		visiblePreviewLines := clipLines(previewLines, m.wizardPreviewScroll, innerH)
		previewContent := strings.Join(visiblePreviewLines, "\n")

		wizardPanelStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(1).
			Width(wizardWidth).
			Height(panelHeight)

		wizardPanel := wizardPanelStyle.Render(wizardContent)

		previewPanelStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(1).
			Width(previewWidth).
			Height(panelHeight)

		previewPanel := previewPanelStyle.Render(previewContent)

		switch m.config.Panel.Position {
		case "left":
			sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, wizardPanel, "│", previewPanel))
		case "right":
			sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, previewPanel, "│", wizardPanel))
		case "bottom":
			sb.WriteString(m.renderMainPanels())
			sb.WriteString(wizardPanel)
		default:
			sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, wizardPanel, "│", previewPanel))
		}
	}

	return sb.String()
}

// renderSnippetPreview builds the markdown preview for a specific snipped, used as the right panel during wizard.
func (m *AppModel) renderSnippetPreview(snip domain.Snippet) string {
	var md strings.Builder
	md.WriteString("# " + snip.Name + "\n\n")
	if len(snip.Tags) > 0 {
		md.WriteString("**Tags:** " + strings.Join(snip.Tags, ", ") + "\n\n")
	}
	if snip.Description != "" {
		md.WriteString(snip.Description + "\n\n")
	}
	md.WriteString("## Code\n\n")
	md.WriteString("```")
	if snip.Language != "" {
		md.WriteString(snip.Language)
	}
	md.WriteString("\n" + snip.Body)
	if !strings.HasSuffix(snip.Body, "\n") {
		md.WriteString("\n")
	}
	md.WriteString("```\n")

	renderedMD, _ := mdRenderer.Render(md.String())
	return renderedMD
}

// renderWizardContent builds the wizard panel inner content.
func (m *AppModel) renderWizardContent() string {
	p := m.wizardSnippet.Placeholders[m.wizardVarIndex]

	_, _, panelHeight := m.panelDims()
	innerH := m.innerContentHeight(panelHeight)

	var wizard strings.Builder
	wizard.WriteString(instructionsStyle.Render(fmt.Sprintf("[Fill Template: %d/%d]  Enter: next  Shift+Tab: back  Esc: cancel", m.wizardVarIndex+1, len(m.wizardSnippet.Placeholders))) + "\n\n")
	wizard.WriteString(headerStyle.Render("Snippet: ") + m.wizardSnippet.Name + "\n")
	wizard.WriteString(headerStyle.Render("Variable: ") + p.Name + "\n\n")
	wizard.WriteString(m.wizardInput.View() + "\n\n")
	if m.wizardLoadingSuggs {
		wizard.WriteString("  Loading suggestions...\n")
	} else if m.wizardError != nil {
		wizard.WriteString(fmt.Sprintf("  Warning: suggestion command error: %v\n", m.wizardError))
	} else if len(m.wizardSuggestions) > 0 {
		wizard.WriteString(headerStyle.Render("Suggestions:") + "\n")

		var sugLines []string
		for i, sug := range m.wizardSuggestions {
			if i == m.wizardSugSelectedIndex {
				sugLines = append(sugLines, selectedItemStyle.Render(fmt.Sprintf("  ▸ %s", sug)))
			} else {
				sugLines = append(sugLines, normalItemStyle.Render(fmt.Sprintf("    %s", sug)))
			}
		}
		sugVisibleH := innerH / 3
		if sugVisibleH < 3 {
			sugVisibleH = 3
		}
		visibleSugs := clipLines(sugLines, m.wizardSugScrollOffset, sugVisibleH)
		for _, line := range visibleSugs {
			wizard.WriteString(line + "\n")
		}
	}

	content := wizard.String()
	contentLines := strings.Split(content, "\n")
	visibleLines := clipLines(contentLines, 0, innerH)
	return strings.Join(visibleLines, "\n")
}

func (m *AppModel) renderWizardPreview() string {
	snip := m.wizardSnippet
	p := snip.Placeholders[m.wizardVarIndex]

	previewVals := make(map[string]string)
	for k, v := range m.wizardConfirmed {
		previewVals[k] = v
	}
	if cur := m.wizardInput.Value(); cur != "" {
		previewVals[p.Name] = cur
	}
	r := resolver.NewResolver(m.config, snip, domain.Frontmatter{})
	renderedCode := r.RenderSnippet(previewVals)

	var md strings.Builder
	md.WriteString("# " + snip.Name + "\n\n")
	if len(snip.Tags) > 0 {
		md.WriteString("**Tags:** " + strings.Join(snip.Tags, ", ") + "\n\n")
	}
	if snip.Description != "" {
		md.WriteString(snip.Description + "\n\n")
	}
	md.WriteString("## Code\n\n")
	md.WriteString("```")
	if snip.Language != "" {
		md.WriteString(snip.Language)
	}
	md.WriteString("\n" + renderedCode)
	if !strings.HasSuffix(renderedCode, "\n") {
		md.WriteString("\n")
	}
	md.WriteString("```\n")

	renderedMD, _ := mdRenderer.Render(md.String())
	return renderedMD
}

// renderMainPanels builds the left/right pane view used by Finder and as background for Wizard.
func (m *AppModel) renderMainPanels() string {
	filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())

	var listLines []string
	for i, snip := range filtered {
		label := fmt.Sprintf("  %s (%s)", snip.Name, snip.RelativePath)
		if i == m.finderSelectedIndex {
			listLines = append(listLines, selectedItemStyle.Render("▸"+label[1:]))
		} else {
			listLines = append(listLines, normalItemStyle.Render(label))
		}
	}

	leftWidth, rightWidth, panelHeight := m.panelDims()
	innerH := m.innerContentHeight(panelHeight)

	visibleListLines := clipLines(listLines, m.finderScrollOffset, innerH)
	listContent := strings.Join(visibleListLines, "\n")
	if listContent == "" {
		listContent = "  No matches found."
	}

	previewContent := "No snippet selected."
	if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
		rendered := m.renderSnippetPreview(filtered[m.finderSelectedIndex])
		previewLines := strings.Split(rendered, "\n")

		maxPreviewScroll := len(previewLines) - innerH
		if maxPreviewScroll < 0 {
			maxPreviewScroll = 0
		}
		if m.finderPreviewScroll > maxPreviewScroll {
			m.finderPreviewScroll = maxPreviewScroll
		}

		visiblePreviewLines := clipLines(previewLines, m.finderPreviewScroll, innerH)
		previewContent = strings.Join(visiblePreviewLines, "\n")
	}

	leftStyle := previewBorder
	rightStyle := previewBorder
	if m.finderFocusedPanel == 0 {
		leftStyle = focusedBorder
	} else {
		rightStyle = focusedBorder
	}

	leftPaneStyle := leftStyle.Width(leftWidth).Height(panelHeight)
	leftBox := leftPaneStyle.Render(listContent)
	rightPaneStyle := rightStyle.Width(rightWidth).Height(panelHeight)
	rightBox := rightPaneStyle.Render(previewContent)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, "│", rightBox)
	return joined + "\n"
}
