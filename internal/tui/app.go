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

	// Browser Mode state
	browserPath          string // virtual directory, e.g. "", "nested", "nested/docker"
	browserItems         []browserItem
	browserSelectedIndex int

	// Tags Mode state
	tags                     []string
	tagCounts                map[string]int
	tagsSelectedIndex        int
	selectedTag              string
	tagSnippets              []domain.Snippet
	tagSnippetsSelectedIndex int
	inTagSnippets            bool

	// Wizard Mode state
	wizardSnippet          domain.Snippet
	wizardConfirmed        map[string]string
	wizardVarIndex         int
	wizardInput            textinput.Model
	wizardSuggestions      []string
	wizardSugSelectedIndex int
	wizardLoadingSuggs     bool
	wizardError            error
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
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+t":
			if m.mode != WizardMode {
				m.mode = (m.mode + 1) % 3
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		switch keyStr {
		case "up", "ctrl+k":
			if m.finderSelectedIndex > 0 {
				m.finderSelectedIndex--
			}
		case "down", "ctrl+j":
			filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
			if m.finderSelectedIndex < len(filtered)-1 {
				m.finderSelectedIndex++
			}
		case "enter":
			filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
			if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
				return m, m.selectSnippet(filtered[m.finderSelectedIndex])
			}
		default:
			m.finderQuery, cmd = m.finderQuery.Update(msg)
			m.finderSelectedIndex = 0 // Reset selection index on type
		}
	}

	return m, cmd
}

func (m *AppModel) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
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
					// Drill down virtual path
					m.browserPath = item.path
					m.initBrowser()
				} else if item.isSnip {
					// Select snippet
					return m, m.selectSnippet(item.snip)
				}
			}
		case "esc", "backspace":
			// Go up folder levels
			if m.browserPath != "" {
				idx := strings.LastIndex(m.browserPath, "/")
				if idx == -1 {
					m.browserPath = ""
				} else {
					m.browserPath = m.browserPath[:idx]
				}
				m.initBrowser()
			}
		}
	}
	return m, nil
}

func (m *AppModel) updateTags(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			}
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
				}
			}
		}
	}
	return m, nil
}

func (m *AppModel) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

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
		case "enter", "tab":
			// Lock in current variable
			p := m.wizardSnippet.Placeholders[m.wizardVarIndex]
			val := m.wizardInput.Value()
			m.wizardConfirmed[p.Name] = val

			// Advance to next variable
			m.wizardVarIndex++
			return m, m.setupWizardForCurrentVar()
		case "shift+tab":
			// Go back to previous variable
			if m.wizardVarIndex > 0 {
				m.wizardVarIndex--
				pPrev := m.wizardSnippet.Placeholders[m.wizardVarIndex]
				// Remove current resolved value
				delete(m.wizardConfirmed, pPrev.Name)
				return m, m.setupWizardForCurrentVar()
			}
		case "alt+enter", "ctrl+j":
			// Insert literal newline for multi-line support
			m.wizardInput.SetValue(m.wizardInput.Value() + "\n")
		case "esc":
			// Return to picker
			m.mode = FinderMode
			return m, nil
		default:
			m.wizardInput, cmd = m.wizardInput.Update(msg)
		}
	}

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

    instructionsStyle = lipgloss.NewStyle().
        Italic(true).
        Foreground(lipgloss.Color("#888888"))
)

func (m *AppModel) View() string {
    if m.Done {
        return fmt.Sprintf("Snippet copied to clipboard!\n\n%s\n", m.FinalSnippet)
    }

    var sb strings.Builder

    // Render header title
    sb.WriteString(titleStyle.Render("PEANUTCLONE") + " ")

    switch m.mode {
    case FinderMode:
		// Render the primary finder interface (panes)
		sb.WriteString(instructionsStyle.Render("[Ctrl+T: mode] Enter: pick") + "\n")
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

        listContent := strings.Join(listLines, "\n")
        if listContent == "" {
            listContent = "  Empty directory."
        }
        sb.WriteString(listContent)
    case TagsMode:
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
            listContent := strings.Join(listLines, "\n")
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
            listContent := strings.Join(listLines, "\n")
            sb.WriteString(listContent)
        }
    case WizardMode:
	// First render the background panels (list + preview)
	background := m.renderMainPanels()

	// Build wizard content
	p := m.wizardSnippet.Placeholders[m.wizardVarIndex]
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
		for i, sug := range m.wizardSuggestions {
			if i == m.wizardSugSelectedIndex {
				wizard.WriteString(selectedItemStyle.Render(fmt.Sprintf("  ▸ %s", sug)) + "\n")
			} else {
				wizard.WriteString(normalItemStyle.Render(fmt.Sprintf("    %s", sug)) + "\n")
			}
		}
	}
	// Live preview
	previewVals := make(map[string]string)
	for k, v := range m.wizardConfirmed {
		previewVals[k] = v
	}
	if cur := m.wizardInput.Value(); cur != "" {
		previewVals[p.Name] = cur
	}
	r := resolver.NewResolver(m.config, m.wizardSnippet, domain.Frontmatter{})
	preview := r.RenderSnippet(previewVals)
	wizard.WriteString("\n" + headerStyle.Render("Preview:") + "\n")
	wizard.WriteString(preview + "\n")

	// Style the floating pane
	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Padding(1).
		Width(m.termWidth / 2).
		Height(m.termHeight / 2)
	overlay := overlayStyle.Render(wizard.String())
	// Combine background with overlay using lipgloss overlay

	combined := background + "\n" + overlay
	sb.WriteString(combined)
    }

    return sb.String()
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

    listContent := strings.Join(listLines, "\n")
    if listContent == "" {
        listContent = "  No matches found."
    }

    // Preview pane content
    previewContent := "No snippet selected."
    if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
        snip := filtered[m.finderSelectedIndex]
        var tagsStr string
        if len(snip.Tags) > 0 {
            tagsStr = fmt.Sprintf("Tags: [%s]\n", strings.Join(snip.Tags, ", "))
        }
        previewContent = fmt.Sprintf("%s\n\n%s%s\n\n%s\n%s\n%s",
            headerStyle.Render(snip.Name),
            tagsStr,
            snip.Description,
            headerStyle.Render("Code:"),
            "```"+snip.Language,
            snip.Body,
        )
    }

    // Dynamic pane widths
    leftWidth := m.termWidth / 2
    if leftWidth < 1 {
        leftWidth = 40
    }
    rightWidth := m.termWidth - leftWidth - 1
    if rightWidth < 1 {
        rightWidth = 35
    }
    leftPaneStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#444444")).Padding(1).Width(leftWidth)
    leftBox := leftPaneStyle.Render(listContent)
    rightPaneStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#444444")).Padding(1).Width(rightWidth)
    rightBox := rightPaneStyle.Render(previewContent)
    joined := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, "│", rightBox)
    return joined + "\n"
}
