package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"basin/internal/clipboard"
	"basin/internal/config"
	"basin/internal/domain"
	"basin/internal/parser"
	"basin/internal/resolver"
	"basin/internal/search"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glamourStyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type tuiMode int

const (
	FinderMode tuiMode = iota
	BrowserMode
	TagsMode
	WizardMode
	CommandPaletteMode
)

const overlayMargin = 3

type suggestionMsg struct {
	suggestions []string
	err         error
}

type editorFinishedMsg struct {
	err error
}

type browserItem struct {
	name   string
	isDir  bool
	isFile bool
	isSnip bool
	snip   domain.Snippet
	path   string
}

type command struct {
	name        string
	description string
	action      func() tea.Cmd
}

type floatingPane struct {
	active  bool
	title   string
	content []string
	width   int
	height  int
}

type AppModel struct {
	termWidth    int
	termHeight   int
	mode         tuiMode
	config       *config.Config
	strandRoots  []string
	allSnippets  []domain.Snippet
	snippetFiles []domain.SnippetFile
	err          error

	Done         bool
	FinalSnippet string

	finderQuery         textinput.Model
	finderSelectedIndex int
	finderScrollOffset  int
	finderFocusedPanel  int

	browserPath          string
	browserItems         []browserItem
	browserSelectedIndex int
	browserScrollOffset  int
	browserFocusedPanel  int
	browserPreviewScroll int

	tags                     []string
	tagCounts                map[string]int
	tagsSelectedIndex        int
	tagsScrollOffset         int
	tagsFocusedPanel         int
	tagsPreviewScroll        int
	selectedTag              string
	tagSnippets              []domain.Snippet
	tagSnippetsSelectedIndex int
	tagSnippetsScrollOffset  int
	tagSnippetsFocusedPanel  int
	tagSnippetsPreviewScroll int
	inTagSnippets            bool

	wizardSnippet          domain.Snippet
	wizardConfirmed        map[string]string
	wizardVarIndex         int
	wizardInput            textinput.Model
	wizardSuggestions      []string
	wizardSugSelectedIndex int
	wizardSugScrollOffset  int
	wizardLoadingSuggs     bool
	wizardError            error

	commandPaletteQuery    textinput.Model
	commandPaletteSelected int
	commandPaletteCommands []command
	commandPalettePrevMode tuiMode

	finderPreviewScroll int
	wizardPreviewScroll int

	listTopOffset int

	floatingPane floatingPane

	leftCollapsed    bool
	rightCollapsed   bool
	runtimeWidthPct  int
	draggingDivider  bool
	lastMouseX       int
	lastAutoCopyBody string

	mouseDragging bool
	mouseDownRow  int

	clipboardErr    error
	clipboardWarn   string

	filenamePromptActive bool
	filenamePromptInput  textinput.Model
	filenamePromptRoot   string
}

func (m *AppModel) showFloatingPane(title string, content []string, width, height int) {
	m.floatingPane.active = true
	m.floatingPane.title = title
	m.floatingPane.content = content
	m.floatingPane.width = width
	m.floatingPane.height = height
}

func (m *AppModel) hideFloatingPane() {
	m.floatingPane.active = false
}

func NewAppModel(cfg *config.Config, roots []string, snippets []domain.Snippet, files []domain.SnippetFile) *AppModel {
	ti := textinput.New()
	ti.Placeholder = "Type to search strands..."
	ti.Focus()

	wInput := textinput.New()
	wInput.Focus()

	cpInput := textinput.New()
	cpInput.Placeholder = "Type a command..."
	cpInput.Focus()

	fpInput := textinput.New()
	fpInput.Placeholder = "strand-name"

	m := &AppModel{
		mode:                   FinderMode,
		config:                 cfg,
		strandRoots:            roots,
		allSnippets:            snippets,
		snippetFiles:           files,
		finderQuery:            ti,
		wizardInput:            wInput,
		wizardConfirmed:        make(map[string]string),
		commandPaletteQuery:    cpInput,
		commandPaletteSelected: 0,
		filenamePromptInput:    fpInput,
		listTopOffset:          4,
		runtimeWidthPct:        cfg.Panel.WidthPercent,
	}

	m.initBrowser()
	m.initTags()
	m.initCommands()

	if cfg.Output.Clipboard || cfg.Behavior.AutoCopyOnSelect {
		if tool, err := clipboard.DetectClipboardTool(); err != nil {
			m.clipboardWarn = fmt.Sprintf("Clipboard unavailable: %v", err)
		} else {
			_ = tool
		}
	}

	return m
}

func (m *AppModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *AppModel) initBrowser() {
	m.browserItems = m.getBrowserItemsForPath(m.browserPath)
	m.browserSelectedIndex = 0
}

func (m *AppModel) getBrowserItemsForPath(dir string) []browserItem {
	var items []browserItem
	seen := make(map[string]bool)

	dir = strings.Trim(dir, "/")

	for _, file := range m.snippetFiles {
		rel := strings.Trim(file.RelativePath, "/")

		if dir == "" {
			idx := strings.Index(rel, "/")
			if idx == -1 {
				items = append(items, browserItem{name: rel, isFile: true, path: rel})
			} else {
				dirPrefix := rel[:idx]
				if !seen[dirPrefix] {
					seen[dirPrefix] = true
					items = append(items, browserItem{name: dirPrefix, isDir: true, path: dirPrefix})
				}
			}
		} else {
			if strings.HasPrefix(rel, dir+"/") {
				sub := strings.TrimPrefix(rel, dir+"/")
				idx := strings.Index(sub, "/")
				if idx == -1 {
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

func (m *AppModel) initCommands() {
	m.commandPaletteCommands = []command{
		{
			name:        "Finder Mode",
			description: "Switch to fuzzy finder mode",
			action: func() tea.Cmd {
				m.mode = FinderMode
				m.finderFocusedPanel = 0
				m.finderQuery.Focus()
				return nil
			},
		},
		{
			name:        "Browser Mode",
			description: "Switch to directory browser mode",
			action: func() tea.Cmd {
				m.mode = BrowserMode
				m.browserFocusedPanel = 0
				return nil
			},
		},
		{
			name:        "Tags Mode",
			description: "Switch to tags dashboard mode",
			action: func() tea.Cmd {
				m.mode = TagsMode
				m.tagsFocusedPanel = 0
				m.inTagSnippets = false
				return nil
			},
		},
		{
			name:        "Start Wizard",
			description: "Start the variable wizard for selected strand",
			action: func() tea.Cmd {
				var snip domain.Snippet
				switch m.commandPalettePrevMode {
				case FinderMode:
					filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
					if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
						snip = filtered[m.finderSelectedIndex]
					}
				case BrowserMode:
					if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
						item := m.browserItems[m.browserSelectedIndex]
						if item.isSnip {
							snip = item.snip
						}
					}
				case TagsMode:
					if m.inTagSnippets && len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
						snip = m.tagSnippets[m.tagSnippetsSelectedIndex]
					}
				}
				if snip.Name != "" {
					return m.selectSnippet(snip)
				}
				return nil
			},
		},
		{
			name:        "Edit Source File",
			description: "Open the source markdown file in your editor",
			action: func() tea.Cmd {
				return m.openEditorForCurrentStrand()
			},
		},
		{
			name:        "New Strand File",
			description: "Create a new markdown file with sample frontmatter",
			action: func() tea.Cmd {
				if len(m.strandRoots) == 0 {
					return nil
				}
				m.filenamePromptRoot = m.strandRoots[0]
				m.filenamePromptActive = true
				m.filenamePromptInput.SetValue("")
				m.filenamePromptInput.Focus()
				m.mode = m.commandPalettePrevMode
				return nil
			},
		},
		{
			name:        "Quit",
			description: "Exit the application",
			action: func() tea.Cmd {
				return tea.Quit
			},
		},
	}
}

func (m *AppModel) selectSnippet(snip domain.Snippet) tea.Cmd {
	if len(snip.Placeholders) == 0 {
		return m.finalizeSnippet(snip.Body)
	}

	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.wizardInput.Focus()
	return m.setupWizardForCurrentVar()
}

func (m *AppModel) frontmatterForSnippet(snip domain.Snippet) domain.Frontmatter {
	for _, file := range m.snippetFiles {
		if file.Path == snip.SourceFile {
			return file.Frontmatter
		}
	}
	return domain.Frontmatter{}
}

func (m *AppModel) setupWizardForCurrentVar() tea.Cmd {
	fm := m.frontmatterForSnippet(m.wizardSnippet)

	if m.wizardVarIndex >= len(m.wizardSnippet.Placeholders) {
		r := resolver.NewResolver(m.config, m.wizardSnippet, fm)
		rendered := r.RenderSnippet(m.wizardConfirmed)
		return m.finalizeSnippet(rendered)
	}

	p := m.wizardSnippet.Placeholders[m.wizardVarIndex]
	r := resolver.NewResolver(m.config, m.wizardSnippet, fm)

	defVal := r.GetDefaultValue(p)
	m.wizardInput.SetValue(defVal)
	m.wizardInput.Placeholder = fmt.Sprintf("Value for %s...", p.Name)
	m.wizardInput.Width = 30

	m.wizardSuggestions = nil
	m.wizardSugSelectedIndex = 0
	m.wizardError = nil

	// Check if this variable has a command source (from frontmatter Placeholder or config)
	isCmd := p.IsCommand
	if !isCmd {
		if cfgVar, ok := m.config.Variables[p.Name]; ok && cfgVar.Command != "" {
			isCmd = true
		}
	}

	if isCmd {
		m.wizardLoadingSuggs = true
		return func() tea.Msg {
			res := resolver.NewResolver(m.config, m.wizardSnippet, fm)
			suggs, err := res.GetSuggestions(p, m.wizardConfirmed)
			return suggestionMsg{suggestions: suggs, err: err}
		}
	}

	suggs, _ := r.GetSuggestions(p, m.wizardConfirmed)
	m.wizardSuggestions = suggs
	m.wizardLoadingSuggs = false
	return nil
}

func (m *AppModel) finalizeSnippet(body string) tea.Cmd {
	m.Done = true
	m.FinalSnippet = body
	if m.config.Output.Clipboard {
		if err := clipboard.WriteAll(body); err != nil {
			m.clipboardErr = fmt.Errorf("clipboard copy failed: %w", err)
		}
	}
	return tea.Quit
}

func (m *AppModel) autoCopyCurrentSnippet() {
	if !m.config.Behavior.AutoCopyOnSelect {
		return
	}
	var body string
	switch m.mode {
	case FinderMode:
		filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
		if m.finderSelectedIndex >= 0 && m.finderSelectedIndex < len(filtered) {
			body = filtered[m.finderSelectedIndex].Body
		}
	case BrowserMode:
		if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
			item := m.browserItems[m.browserSelectedIndex]
			if item.isSnip {
				body = item.snip.Body
			}
		}
	case TagsMode:
		if m.inTagSnippets && m.tagSnippetsSelectedIndex >= 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
			body = m.tagSnippets[m.tagSnippetsSelectedIndex].Body
		}
	}
	if body != "" && body != m.lastAutoCopyBody {
		m.lastAutoCopyBody = body
		if err := clipboard.WriteAll(body); err != nil {
			m.clipboardErr = fmt.Errorf("auto-copy failed: %w", err)
		}
	}
}

func (m *AppModel) copyPreviewRange(row1, row2 int) {
	start, end := row1, row2
	if start > end {
		start, end = end, start
	}
	start -= m.listTopOffset
	end -= m.listTopOffset

	var previewScroll int
	var rendered string
	switch m.mode {
	case FinderMode:
		filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
		if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
			rendered = m.renderSnippetPreview(filtered[m.finderSelectedIndex])
			previewScroll = m.finderPreviewScroll
		}
	case BrowserMode:
		if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
			item := m.browserItems[m.browserSelectedIndex]
			if item.isSnip {
				rendered = m.renderSnippetPreview(item.snip)
				previewScroll = m.browserPreviewScroll
			}
		}
	case TagsMode:
		if m.inTagSnippets && len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
			rendered = m.renderSnippetPreview(m.tagSnippets[m.tagSnippetsSelectedIndex])
			previewScroll = m.tagSnippetsPreviewScroll
		}
	}
	if rendered == "" {
		return
	}
	lines := strings.Split(rendered, "\n")

	visibleStart := previewScroll + start
	visibleEnd := previewScroll + end
	if visibleStart < 0 {
		visibleStart = 0
	}
	if visibleEnd >= len(lines) {
		visibleEnd = len(lines) - 1
	}
	if visibleStart > visibleEnd {
		return
	}

	var selected strings.Builder
	for i := visibleStart; i <= visibleEnd; i++ {
		if i > visibleStart {
			selected.WriteString("\n")
		}
		selected.WriteString(ansi.Strip(lines[i]))
	}
	if err := clipboard.WriteAll(selected.String()); err != nil {
		m.clipboardErr = fmt.Errorf("preview copy failed: %w", err)
	}
}

func getEditorCommand() (string, []string) {
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor, nil
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor, nil
	}
	switch runtime.GOOS {
	case "windows":
		return "notepad", nil
	default:
		return "vi", nil
	}
}

func (m *AppModel) getSourceFileForCurrentSelection() string {
	switch m.mode {
	case FinderMode:
		filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
		if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
			return filtered[m.finderSelectedIndex].SourceFile
		}
	case BrowserMode:
		if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
			item := m.browserItems[m.browserSelectedIndex]
			if item.isSnip {
				return item.snip.SourceFile
			}
			if item.isFile {
				for _, f := range m.snippetFiles {
					rel := strings.Trim(f.RelativePath, "/")
					if rel == item.path {
						return f.Path
					}
				}
			}
		}
	case TagsMode:
		if m.inTagSnippets && len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
			return m.tagSnippets[m.tagSnippetsSelectedIndex].SourceFile
		}
	case WizardMode:
		return m.wizardSnippet.SourceFile
	}
	return ""
}

func (m *AppModel) openEditorForCurrentStrand() tea.Cmd {
	filePath := m.getSourceFileForCurrentSelection()
	if filePath == "" {
		return nil
	}

	editorCmd, editorArgs := getEditorCommand()
	args := append(editorArgs, filePath)
	c := exec.Command(editorCmd, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m *AppModel) reloadStrands() {
	if len(m.strandRoots) == 0 {
		return
	}

	files, err := parser.DiscoverAndParse(m.strandRoots)
	if err != nil {
		return
	}

	var allSnippets []domain.Snippet
	for _, f := range files {
		allSnippets = append(allSnippets, f.Snippets...)
	}

	m.snippetFiles = files
	m.allSnippets = allSnippets

	m.retainSelectionAfterReload()

	m.initBrowser()
	m.initTags()
}

func (m *AppModel) retainSelectionAfterReload() {
	switch m.mode {
	case FinderMode:
		filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
		if m.finderSelectedIndex >= len(filtered) {
			m.finderSelectedIndex = 0
		}
	case BrowserMode:
		m.browserItems = m.getBrowserItemsForPath(m.browserPath)
		if m.browserSelectedIndex >= len(m.browserItems) {
			m.browserSelectedIndex = 0
		}
	case TagsMode:
		if m.inTagSnippets && m.selectedTag != "" {
			m.filterSnippetsByTag(m.selectedTag)
			if m.tagSnippetsSelectedIndex >= len(m.tagSnippets) {
				m.tagSnippetsSelectedIndex = 0
			}
		}
	}
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.floatingPane.active {
			switch msg.String() {
			case "esc", "enter", "q":
				m.hideFloatingPane()
				return m, nil
			}
			return m, nil
		}
		if m.filenamePromptActive {
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.filenamePromptInput.Value())
				if name == "" {
					name = "strand-name"
				}
				if !strings.HasSuffix(name, ".md") {
					name += ".md"
				}
				filePath := filepath.Join(m.filenamePromptRoot, name)
				content := `---
name: New Strands
tags: [new]
---

## Example strand

Description of what this does.

` + "```sh" + `
echo <$message>
` + "```" + `
`
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					m.filenamePromptActive = false
					return m, nil
				}
				m.filenamePromptActive = false
				editorCmd, editorArgs := getEditorCommand()
				args := append(editorArgs, filePath)
				c := exec.Command(editorCmd, args...)
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return editorFinishedMsg{err: err}
				})
			case "esc":
				m.filenamePromptActive = false
				return m, nil
			default:
				var cmd tea.Cmd
				m.filenamePromptInput, cmd = m.filenamePromptInput.Update(msg)
				return m, cmd
			}
		}
		switch msg.String() {
		case "ctrl+d":
			return m, tea.Quit
		case "ctrl+t":
			if m.mode != WizardMode && m.mode != CommandPaletteMode {
				m.mode = (m.mode + 1) % 3
				m.finderFocusedPanel = 0
				m.finderQuery.Focus()
				m.browserFocusedPanel = 0
				m.tagsFocusedPanel = 0
				m.tagSnippetsFocusedPanel = 0
				return m, nil
			}
		case "ctrl+p":
			if m.mode != CommandPaletteMode {
				m.commandPalettePrevMode = m.mode
				m.mode = CommandPaletteMode
				m.commandPaletteQuery.SetValue("")
				m.commandPaletteSelected = 0
				m.commandPaletteQuery.Focus()
				return m, nil
			}
		case "ctrl+e":
			if m.mode != WizardMode && m.mode != CommandPaletteMode {
				return m, m.openEditorForCurrentStrand()
			}
		case "ctrl+f":
			m.showFloatingPane("Floating Pane", []string{
				"Press Esc, Enter, or Q to dismiss.",
			}, 50, 12)
			return m, nil
		case "[", "{":
			if msg.String() == "{" {
				m.leftCollapsed = !m.leftCollapsed
			} else {
				m.runtimeWidthPct -= 3
				if m.runtimeWidthPct < 5 {
					m.runtimeWidthPct = 5
				}
			}
			return m, nil
		case "]", "}":
			if msg.String() == "}" {
				m.rightCollapsed = !m.rightCollapsed
			} else {
				m.runtimeWidthPct += 3
				if m.runtimeWidthPct > 90 {
					m.runtimeWidthPct = 90
				}
			}
			return m, nil
		}
	case tea.MouseMsg:
		return m.handleMouse(msg)
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
	case editorFinishedMsg:
		m.reloadStrands()
		return m, nil
	}

	switch m.mode {
	case FinderMode:
		return m.updateFinder(msg)
	case BrowserMode:
		return m.updateBrowser(msg)
	case TagsMode:
		return m.updateTags(msg)
	case WizardMode:
		return m.updateWizard(msg)
	case CommandPaletteMode:
		return m.updateCommandPalette(msg)
	}

	return m, nil
}

func (m *AppModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	row := msg.Y
	col := msg.X
	leftWidth, _, panelHeight := m.panelDims()
	innerH := m.innerContentHeight(panelHeight)
	dividerCol := leftWidth + 2

	if msg.Action == tea.MouseActionRelease {
		m.draggingDivider = false
		if m.mouseDragging && msg.Button == tea.MouseButtonLeft {
			m.mouseDragging = false
			if m.mode != WizardMode && m.mode != CommandPaletteMode {
				if col >= leftWidth+2 && row >= m.listTopOffset && row < m.listTopOffset+innerH {
					m.copyPreviewRange(m.mouseDownRow, row)
				}
			}
		}
		return m, nil
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && col >= leftWidth+2 && row >= m.listTopOffset && row < m.listTopOffset+innerH && m.mode != WizardMode && m.mode != CommandPaletteMode {
		m.mouseDragging = true
		m.mouseDownRow = row
		return m, nil
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && col >= dividerCol-1 && col <= dividerCol+1 && row >= 3 && row < 3+panelHeight {
		m.draggingDivider = true
		m.lastMouseX = col
		return m, nil
	}

	if m.draggingDivider && (msg.Action == tea.MouseActionMotion || msg.Action == tea.MouseActionPress) {
		if col != m.lastMouseX {
			newPct := (col * 100) / m.termWidth
			if newPct < 5 {
				newPct = 5
			}
			if newPct > 90 {
				newPct = 90
			}
			m.runtimeWidthPct = newPct
			m.leftCollapsed = false
			m.rightCollapsed = false
			m.lastMouseX = col
		}
		return m, nil
	}

	if msg.Action == tea.MouseActionMotion && m.mouseDragging {
		return m, nil
	}

	if msg.Action != tea.MouseActionPress {
		return m, nil
	}

	switch m.mode {
	case FinderMode:
		filtered := search.FilterSnippets(m.allSnippets, m.finderQuery.Value())
		if col < leftWidth+2 && row >= m.listTopOffset && row < m.listTopOffset+innerH {
			idx := m.finderScrollOffset + (row - m.listTopOffset)
			if idx >= 0 && idx < len(filtered) {
				m.finderSelectedIndex = idx
				m.finderPreviewScroll = 0
				m.finderFocusedPanel = 0
				m.finderQuery.Focus()
				m.autoCopyCurrentSnippet()
			}
		} else if col >= leftWidth+2 && row >= m.listTopOffset {
			m.finderFocusedPanel = 1
			m.finderQuery.Blur()
			if msg.Button == tea.MouseButtonWheelUp {
				m.finderPreviewScroll--
				if m.finderPreviewScroll < 0 {
					m.finderPreviewScroll = 0
				}
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.finderPreviewScroll++
			}
		}
		if msg.Button == tea.MouseButtonWheelUp && col < leftWidth+2 {
			if m.finderSelectedIndex > 0 {
				m.finderSelectedIndex--
				m.finderPreviewScroll = 0
				m.autoCopyCurrentSnippet()
			}
		} else if msg.Button == tea.MouseButtonWheelDown && col < leftWidth+2 {
			if m.finderSelectedIndex < len(filtered)-1 {
				m.finderSelectedIndex++
				m.finderPreviewScroll = 0
				m.autoCopyCurrentSnippet()
			}
		}

	case BrowserMode:
		if col < leftWidth+2 && row >= m.listTopOffset && row < m.listTopOffset+innerH {
			idx := m.browserScrollOffset + (row - m.listTopOffset)
			if idx >= 0 && idx < len(m.browserItems) {
				m.browserSelectedIndex = idx
				m.browserPreviewScroll = 0
				m.browserFocusedPanel = 0
				m.autoCopyCurrentSnippet()
			}
		} else if col >= leftWidth+2 {
			m.browserFocusedPanel = 1
			if msg.Button == tea.MouseButtonWheelUp {
				m.browserPreviewScroll--
				if m.browserPreviewScroll < 0 {
					m.browserPreviewScroll = 0
				}
			} else if msg.Button == tea.MouseButtonWheelDown {
				m.browserPreviewScroll++
			}
		}
		if msg.Button == tea.MouseButtonWheelUp && col < leftWidth+2 {
			if m.browserSelectedIndex > 0 {
				m.browserSelectedIndex--
				m.browserPreviewScroll = 0
				m.autoCopyCurrentSnippet()
			}
		} else if msg.Button == tea.MouseButtonWheelDown && col < leftWidth+2 {
			if m.browserSelectedIndex < len(m.browserItems)-1 {
				m.browserSelectedIndex++
				m.browserPreviewScroll = 0
				m.autoCopyCurrentSnippet()
			}
		}

	case TagsMode:
		if m.inTagSnippets {
			if col < leftWidth+2 && row >= m.listTopOffset && row < m.listTopOffset+innerH {
				idx := m.tagSnippetsScrollOffset + (row - m.listTopOffset)
				if idx >= 0 && idx < len(m.tagSnippets) {
					m.tagSnippetsSelectedIndex = idx
					m.tagSnippetsPreviewScroll = 0
					m.tagSnippetsFocusedPanel = 0
					m.autoCopyCurrentSnippet()
				}
			} else if col >= leftWidth+2 {
				m.tagSnippetsFocusedPanel = 1
				if msg.Button == tea.MouseButtonWheelUp {
					m.tagSnippetsPreviewScroll--
					if m.tagSnippetsPreviewScroll < 0 {
						m.tagSnippetsPreviewScroll = 0
					}
				} else if msg.Button == tea.MouseButtonWheelDown {
					m.tagSnippetsPreviewScroll++
				}
			}
		} else {
			if col < leftWidth+2 && row >= m.listTopOffset && row < m.listTopOffset+innerH {
				idx := m.tagsScrollOffset + (row - m.listTopOffset)
				if idx >= 0 && idx < len(m.tags) {
					m.tagsSelectedIndex = idx
					m.tagsPreviewScroll = 0
					m.tagsFocusedPanel = 0
				}
			} else if col >= leftWidth+2 {
				m.tagsFocusedPanel = 1
				if msg.Button == tea.MouseButtonWheelUp {
					m.tagsPreviewScroll--
					if m.tagsPreviewScroll < 0 {
						m.tagsPreviewScroll = 0
					}
				} else if msg.Button == tea.MouseButtonWheelDown {
					m.tagsPreviewScroll++
				}
			}
		}

	case WizardMode:
		if msg.Button == tea.MouseButtonWheelUp {
			if col >= leftWidth+2 {
				m.wizardPreviewScroll--
				if m.wizardPreviewScroll < 0 {
					m.wizardPreviewScroll = 0
				}
			}
		} else if msg.Button == tea.MouseButtonWheelDown {
			if col >= leftWidth+2 {
				m.wizardPreviewScroll++
			}
		}

	case CommandPaletteMode:
		filtered := m.getFilteredCommands()
		if row >= m.listTopOffset && row < m.listTopOffset+10 {
			idx := m.commandPaletteSelected
			start := m.commandPaletteSelected - 5
			if start < 0 {
				start = 0
			}
			relRow := row - m.listTopOffset
			clickedIdx := start + relRow
			if clickedIdx >= 0 && clickedIdx < len(filtered) {
				idx = clickedIdx
				m.commandPaletteSelected = idx
			}
		}
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
	m.autoCopyCurrentSnippet()

	return m, cmd
}

func (m *AppModel) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, _, panelHeight := m.panelDims()
	visibleH := m.innerContentHeight(panelHeight)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()

		if m.browserFocusedPanel == 1 {
			switch keyStr {
			case "up", "ctrl+k":
				m.browserPreviewScroll--
				if m.browserPreviewScroll < 0 {
					m.browserPreviewScroll = 0
				}
			case "down", "ctrl+j":
				m.browserPreviewScroll++
			case "ctrl+u":
				m.browserPreviewScroll -= visibleH / 2
				if m.browserPreviewScroll < 0 {
					m.browserPreviewScroll = 0
				}
			case "left", "esc", "tab":
				m.browserFocusedPanel = 0
			case "ctrl+c":
				if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
					item := m.browserItems[m.browserSelectedIndex]
					if item.isSnip {
						return m, m.selectSnippet(item.snip)
					}
				}
			}
			return m, nil
		}

		switch keyStr {
		case "up", "ctrl+k":
			if m.browserSelectedIndex > 0 {
				m.browserSelectedIndex--
				m.browserPreviewScroll = 0
			}
		case "down", "ctrl+j":
			if m.browserSelectedIndex < len(m.browserItems)-1 {
				m.browserSelectedIndex++
				m.browserPreviewScroll = 0
			}
		case "enter", "tab":
			if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
				item := m.browserItems[m.browserSelectedIndex]
				if item.isDir || item.isFile {
					m.browserPath = item.path
					m.initBrowser()
					m.browserScrollOffset = 0
					m.browserPreviewScroll = 0
				} else if item.isSnip {
					return m, m.selectSnippet(item.snip)
				}
			}
		case "ctrl+c":
			if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
				item := m.browserItems[m.browserSelectedIndex]
				if item.isSnip {
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
				m.browserPreviewScroll = 0
			}
		}
	}

	m.browserScrollOffset = clampScroll(m.browserSelectedIndex, m.browserScrollOffset, visibleH)
	m.autoCopyCurrentSnippet()

	return m, nil
}

func (m *AppModel) updateTags(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, _, panelHeight := m.panelDims()
	visibleH := m.innerContentHeight(panelHeight)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		if m.inTagSnippets {
			if m.tagSnippetsFocusedPanel == 1 {
				switch keyStr {
				case "up", "ctrl+k":
					m.tagSnippetsPreviewScroll--
					if m.tagSnippetsPreviewScroll < 0 {
						m.tagSnippetsPreviewScroll = 0
					}
				case "down", "ctrl+j":
					m.tagSnippetsPreviewScroll++
				case "ctrl+u":
					m.tagSnippetsPreviewScroll -= visibleH / 2
					if m.tagSnippetsPreviewScroll < 0 {
						m.tagSnippetsPreviewScroll = 0
					}
				case "left", "esc", "tab":
					m.tagSnippetsFocusedPanel = 0
				case "ctrl+c":
					if len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
						return m, m.selectSnippet(m.tagSnippets[m.tagSnippetsSelectedIndex])
					}
				}
				return m, nil
			}

			switch keyStr {
			case "up", "ctrl+k":
				if m.tagSnippetsSelectedIndex > 0 {
					m.tagSnippetsSelectedIndex--
					m.tagSnippetsPreviewScroll = 0
				}
			case "down", "ctrl+j":
				if m.tagSnippetsSelectedIndex < len(m.tagSnippets)-1 {
					m.tagSnippetsSelectedIndex++
					m.tagSnippetsPreviewScroll = 0
				}
			case "enter", "tab":
				if len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
					return m, m.selectSnippet(m.tagSnippets[m.tagSnippetsSelectedIndex])
				}
			case "ctrl+c":
				if len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
					return m, m.selectSnippet(m.tagSnippets[m.tagSnippetsSelectedIndex])
				}
			case "esc", "backspace":
				m.inTagSnippets = false
				m.tagSnippetsScrollOffset = 0
				m.tagSnippetsPreviewScroll = 0
			}
			m.tagSnippetsScrollOffset = clampScroll(m.tagSnippetsSelectedIndex, m.tagSnippetsScrollOffset, visibleH)
		} else {
			if m.tagsFocusedPanel == 1 {
				switch keyStr {
				case "up", "ctrl+k":
					m.tagsPreviewScroll--
					if m.tagsPreviewScroll < 0 {
						m.tagsPreviewScroll = 0
					}
				case "down", "ctrl+j":
					m.tagsPreviewScroll++
				case "ctrl+u":
					m.tagsPreviewScroll -= visibleH / 2
					if m.tagsPreviewScroll < 0 {
						m.tagsPreviewScroll = 0
					}
				case "left", "esc", "tab":
					m.tagsFocusedPanel = 0
				}
				return m, nil
			}

			switch keyStr {
			case "up", "ctrl+k":
				if m.tagsSelectedIndex > 0 {
					m.tagsSelectedIndex--
					m.tagsPreviewScroll = 0
				}
			case "down", "ctrl+j":
				if m.tagsSelectedIndex < len(m.tags)-1 {
					m.tagsSelectedIndex++
					m.tagsPreviewScroll = 0
				}
			case "enter", "tab":
				if len(m.tags) > 0 && m.tagsSelectedIndex < len(m.tags) {
					m.selectedTag = m.tags[m.tagsSelectedIndex]
					m.filterSnippetsByTag(m.selectedTag)
					m.inTagSnippets = true
					m.tagSnippetsScrollOffset = 0
					m.tagSnippetsPreviewScroll = 0
				}
			}
			m.tagsScrollOffset = clampScroll(m.tagsSelectedIndex, m.tagsScrollOffset, visibleH)
		}
	}
	m.autoCopyCurrentSnippet()
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

	return m, cmd
}

func (m *AppModel) updateCommandPalette(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	filtered := m.getFilteredCommands()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		switch keyStr {
		case "up", "ctrl+k":
			if m.commandPaletteSelected > 0 {
				m.commandPaletteSelected--
			}
		case "down", "ctrl+j":
			if m.commandPaletteSelected < len(filtered)-1 {
				m.commandPaletteSelected++
			}
		case "enter":
			if len(filtered) > 0 && m.commandPaletteSelected < len(filtered) {
				action := filtered[m.commandPaletteSelected].action
				return m, action()
			}
		case "esc":
			m.mode = m.commandPalettePrevMode
			switch m.mode {
			case FinderMode:
				m.finderQuery.Focus()
			case BrowserMode:
			case TagsMode:
			}
			return m, nil
		default:
			m.commandPaletteQuery, cmd = m.commandPaletteQuery.Update(msg)
			m.commandPaletteSelected = 0
		}
	}

	return m, cmd
}

func (m *AppModel) getFilteredCommands() []command {
	query := strings.ToLower(m.commandPaletteQuery.Value())
	if query == "" {
		return m.commandPaletteCommands
	}

	var filtered []command
	for _, cmd := range m.commandPaletteCommands {
		if strings.Contains(strings.ToLower(cmd.name), query) ||
			strings.Contains(strings.ToLower(cmd.description), query) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

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

	basinGlamourStyle = func() glamour.TermRendererOption {
		s := glamourStyles.DarkStyleConfig
		codeBlockBg := "#262626"
		codeBlockFg := "#C9C9C9"
		s.CodeBlock.BackgroundColor = &codeBlockBg
		s.CodeBlock.Color = &codeBlockFg
		zeroMargin := uint(0)
		s.CodeBlock.Margin = &zeroMargin
		inlineCodeBg := "#303030"
		inlineCodeFg := "#E5C07B"
		s.Code.BackgroundColor = &inlineCodeBg
		s.Code.Color = &inlineCodeFg
		return glamour.WithStyles(s)
	}

	mdRenderer, _ = glamour.NewTermRenderer(
		basinGlamourStyle(),
		glamour.WithWordWrap(0),
	)
)

func (m *AppModel) panelDims() (leftWidth, rightWidth, height int) {
	const footerRows = 1
	headerRows := m.headerHeight()
	availHeight := m.termHeight - headerRows - footerRows
	if availHeight < 1 {
		availHeight = 1
	}
	height = availHeight * m.config.Panel.HeightPercent / 100
	if height < 1 {
		height = 1
	}
	if height > availHeight {
		height = availHeight
	}

	if m.leftCollapsed && m.rightCollapsed {
		leftWidth = m.termWidth / 2
		rightWidth = m.termWidth - leftWidth - 2
	} else if m.leftCollapsed {
		leftWidth = 3
		rightWidth = m.termWidth - leftWidth - 2
	} else if m.rightCollapsed {
		rightWidth = 3
		leftWidth = m.termWidth - rightWidth - 2
	} else {
		leftWidth = m.termWidth * m.runtimeWidthPct / 100
		if leftWidth < 1 {
			leftWidth = 40
		}
		rightWidth = m.termWidth - leftWidth - 2
		if rightWidth < 1 {
			rightWidth = 20
		}
	}
	if rightWidth < 1 {
		rightWidth = 1
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

func (m *AppModel) paneContentHeight(panelHeight int) int {
	h := panelHeight - 2
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

func renderScrollbar(totalLines, scrollOffset, visibleHeight, panelHeight int) string {
	if totalLines <= visibleHeight {
		return ""
	}

	innerH := panelHeight - 4
	if innerH < 1 {
		innerH = 1
	}

	thumbSize := (visibleHeight * innerH) / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}

	maxScroll := totalLines - visibleHeight
	if maxScroll <= 0 {
		return ""
	}

	thumbPos := (scrollOffset * (innerH - thumbSize)) / maxScroll
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > innerH {
		thumbPos = innerH - thumbSize
	}

	var sb strings.Builder
	for i := 0; i < innerH; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString("█")
		} else {
			sb.WriteString("░")
		}
		if i < innerH-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (m *AppModel) View() string {
	if m.Done {
		if m.clipboardErr != nil {
			tool, _ := clipboard.DetectClipboardTool()
			return fmt.Sprintf("Clipboard error: %v\nDetected tool: %s\nDISPLAY=%s WAYLAND_DISPLAY=%s\n\n%s\n",
				m.clipboardErr, tool, os.Getenv("DISPLAY"), os.Getenv("WAYLAND_DISPLAY"), m.FinalSnippet)
		}
		return fmt.Sprintf("Strand copied to clipboard!\n\n%s\n", m.FinalSnippet)
	}

	var sb strings.Builder

	header := m.renderHeader()
	footer := m.renderFooter()

	sb.WriteString(header)

	switch m.mode {
	case FinderMode:
		sb.WriteString(m.renderMainPanels())
	case BrowserMode:
		sb.WriteString(m.renderBrowserPanels())
	case TagsMode:
		sb.WriteString(m.renderTagsPanels())
	case WizardMode:
		sb.WriteString(m.renderMainPanels())
	case CommandPaletteMode:
		// Show previous mode's panels underneath the overlay
		switch m.commandPalettePrevMode {
		case FinderMode:
			sb.WriteString(m.renderMainPanels())
		case BrowserMode:
			sb.WriteString(m.renderBrowserPanels())
		case TagsMode:
			sb.WriteString(m.renderTagsPanels())
		default:
			sb.WriteString(m.renderMainPanels())
		}
	}

	sb.WriteString(footer)
	baseView := sb.String()

	// Overlay wizard dialog
	if m.mode == WizardMode {
		overlayContent := m.renderWizardOverlay()
		if overlayContent != "" {
			baseView = m.overlayOnView(baseView, overlayContent)
		}
	}

	// Overlay command palette
	if m.mode == CommandPaletteMode {
		overlayContent := m.renderCommandPaletteOverlay()
		if overlayContent != "" {
			baseView = m.overlayOnView(baseView, overlayContent)
		}
	}

	// Overlay filename prompt
	if m.filenamePromptActive {
		overlayContent := m.renderFilenamePromptOverlay()
		if overlayContent != "" {
			baseView = m.overlayOnView(baseView, overlayContent)
		}
	}

	if m.floatingPane.active {
		return m.renderWithFloatingPane(baseView)
	}

	return baseView
}

func (m *AppModel) renderHeader() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("BASIN") + " ")

	switch m.mode {
	case FinderMode:
		sb.WriteString(titleStyle.Render("Fuzzyfinder") + "\n")
		sb.WriteString(m.finderQuery.View() + "\n")
	case BrowserMode:
		sb.WriteString(titleStyle.Render(fmt.Sprintf("Browser: /%s", m.browserPath)) + "\n")
	case TagsMode:
		if m.inTagSnippets {
			sb.WriteString(titleStyle.Render(fmt.Sprintf("Tag: %s", m.selectedTag)) + "\n")
		} else {
			sb.WriteString(titleStyle.Render("Tags") + "\n")
		}
	case WizardMode:
		sb.WriteString(titleStyle.Render("Fuzzyfinder") + "\n")
		sb.WriteString(m.finderQuery.View() + "\n")
	case CommandPaletteMode:
		// Show previous mode's header underneath the overlay
		prevMode := m.commandPalettePrevMode
		switch prevMode {
		case FinderMode:
			sb.WriteString(titleStyle.Render("Fuzzyfinder") + "\n")
			sb.WriteString(m.finderQuery.View() + "\n")
		case BrowserMode:
			sb.WriteString(titleStyle.Render(fmt.Sprintf("Browser: /%s", m.browserPath)) + "\n")
		case TagsMode:
			if m.inTagSnippets {
				sb.WriteString(titleStyle.Render(fmt.Sprintf("Tag: %s", m.selectedTag)) + "\n")
			} else {
				sb.WriteString(titleStyle.Render("Tags") + "\n")
			}
		}
	}

	return sb.String()
}

func (m *AppModel) renderFooter() string {
	var instructions string

	switch m.mode {
	case FinderMode:
		if m.finderFocusedPanel == 0 {
			instructions = "Enter: wizard  Tab: focus preview  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
		} else {
			instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
		}
	case BrowserMode:
		if m.browserFocusedPanel == 0 {
			instructions = "Enter: open/wizard  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
		} else {
			instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
		}
	case TagsMode:
		if m.inTagSnippets {
			if m.tagSnippetsFocusedPanel == 0 {
				instructions = "Enter: wizard  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
			} else {
				instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
			}
		} else {
			if m.tagsFocusedPanel == 0 {
				instructions = "Enter: open  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
			} else {
				instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
			}
		}
	case WizardMode:
		instructions = "Ctrl+F: float  Ctrl+D: quit"
	case CommandPaletteMode:
		// Show previous mode's footer underneath the overlay
		prevMode := m.commandPalettePrevMode
		switch prevMode {
		case FinderMode:
			if m.finderFocusedPanel == 0 {
				instructions = "Enter: wizard  Tab: focus preview  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
			} else {
				instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
			}
		case BrowserMode:
			if m.browserFocusedPanel == 0 {
				instructions = "Enter: open/wizard  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
			} else {
				instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
			}
		case TagsMode:
			if m.inTagSnippets {
				if m.tagSnippetsFocusedPanel == 0 {
					instructions = "Enter: wizard  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
				} else {
					instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
				}
			} else {
				if m.tagsFocusedPanel == 0 {
					instructions = "Enter: open  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit"
				} else {
					instructions = "Tab/Esc/Left: back to list  Ctrl+E: edit  Ctrl+F: float  Ctrl+P: commands  Ctrl+T: mode  Ctrl+D: quit  ↑↓: scroll"
				}
			}
		}
	}

	footerLine := instructionsStyle.Render(instructions)

	result := footerLine + "\n"
	if m.clipboardWarn != "" {
		result += m.clipboardWarn + "\n"
	}
	return result
}

func (m *AppModel) renderCommandPalette() string {
	filtered := m.getFilteredCommands()

	var sb strings.Builder
	sb.WriteString("\n")

	if len(filtered) == 0 {
		sb.WriteString("  No commands found.\n")
		return sb.String()
	}

	maxVisible := 10
	if len(filtered) < maxVisible {
		maxVisible = len(filtered)
	}

	start := m.commandPaletteSelected - maxVisible/2
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > len(filtered) {
		end = len(filtered)
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		cmd := filtered[i]
		if i == m.commandPaletteSelected {
			sb.WriteString(selectedItemStyle.Render("▸ "))
			sb.WriteString(selectedItemStyle.Render(cmd.name))
			sb.WriteString("  ")
			sb.WriteString(instructionsStyle.Render(cmd.description))
		} else {
			sb.WriteString("  ")
			sb.WriteString(normalItemStyle.Render(cmd.name))
			sb.WriteString("  ")
			sb.WriteString(instructionsStyle.Render(cmd.description))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *AppModel) renderCommandPaletteOverlay() string {
	filtered := m.getFilteredCommands()

	var body strings.Builder

	body.WriteString(headerStyle.Render("Command Palette"))
	body.WriteString("\n\n")
	body.WriteString(m.commandPaletteQuery.View())
	body.WriteString("\n\n")

	if len(filtered) == 0 {
		body.WriteString("  No commands found.\n")
	} else {
		visible := m.termHeight - 12
		if visible < 5 {
			visible = 5
		}
		if len(filtered) < visible {
			visible = len(filtered)
		}

		start := m.commandPaletteSelected - visible/2
		if start < 0 {
			start = 0
		}
		end := start + visible
		if end > len(filtered) {
			end = len(filtered)
			start = end - visible
			if start < 0 {
				start = 0
			}
		}

		for i := start; i < end; i++ {
			cmd := filtered[i]
			if i == m.commandPaletteSelected {
				body.WriteString(selectedItemStyle.Render("▸ "))
				body.WriteString(selectedItemStyle.Render(cmd.name))
				body.WriteString("  ")
				body.WriteString(instructionsStyle.Render(cmd.description))
			} else {
				body.WriteString("  ")
				body.WriteString(normalItemStyle.Render(cmd.name))
				body.WriteString("  ")
				body.WriteString(instructionsStyle.Render(cmd.description))
			}
			body.WriteString("\n")
		}
	}

	body.WriteString("\n")
	body.WriteString(instructionsStyle.Render("Enter: execute  Esc: cancel  ↑↓: navigate"))

	overlayWidth := m.termWidth - overlayMargin*2
	if overlayWidth < 40 {
		overlayWidth = 40
	}
	overlayHeight := m.termHeight - overlayMargin*2
	if overlayHeight < 10 {
		overlayHeight = 10
	}

	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#5F5FAF")).
		Background(lipgloss.Color("#1E1E1E")).
		Width(overlayWidth).
		Height(overlayHeight).
		Padding(1, 1)

	return overlayStyle.Render(body.String())
}

func (m *AppModel) renderFilenamePromptOverlay() string {
	var body strings.Builder

	body.WriteString(headerStyle.Render("New Strand File"))
	body.WriteString("\n\n")
	body.WriteString("Enter a filename for the new strand file:\n\n")
	body.WriteString(m.filenamePromptInput.View())
	body.WriteString("\n\n")
	body.WriteString(instructionsStyle.Render("Enter: create  Esc: cancel"))

	overlayWidth := m.termWidth - overlayMargin*2
	if overlayWidth < 40 {
		overlayWidth = 40
	}
	overlayHeight := 12
	if overlayHeight > m.termHeight-overlayMargin*2 {
		overlayHeight = m.termHeight - overlayMargin*2
	}

	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#5F5FAF")).
		Background(lipgloss.Color("#1E1E1E")).
		Width(overlayWidth).
		Height(overlayHeight).
		Padding(1, 1)

	return overlayStyle.Render(body.String())
}

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

func (m *AppModel) renderPreviewForOverlay() string {
	if m.wizardVarIndex >= len(m.wizardSnippet.Placeholders) {
		return ""
	}
	p := m.wizardSnippet.Placeholders[m.wizardVarIndex]

	previewVals := make(map[string]string)
	for k, v := range m.wizardConfirmed {
		previewVals[k] = v
	}
	if cur := m.wizardInput.Value(); cur != "" {
		previewVals[p.Name] = cur
	}
	r := resolver.NewResolver(m.config, m.wizardSnippet, domain.Frontmatter{})
	code := r.RenderSnippet(previewVals)

	lines := strings.Split(code, "\n")
	maxPreview := 5
	if len(lines) > maxPreview {
		lines = lines[:maxPreview]
		lines = append(lines, "...")
	}
	return strings.Join(lines, "\n")
}

func (m *AppModel) renderWizardOverlay() string {
	if m.wizardVarIndex >= len(m.wizardSnippet.Placeholders) {
		return ""
	}
	p := m.wizardSnippet.Placeholders[m.wizardVarIndex]

	var body strings.Builder

	// Title: progress + snippet name + variable
	body.WriteString(headerStyle.Render(fmt.Sprintf("[%d/%d] %s > %s",
		m.wizardVarIndex+1, len(m.wizardSnippet.Placeholders),
		m.wizardSnippet.Name, p.Name)))
	body.WriteString("\n\n")

	// Input line
	body.WriteString(headerStyle.Render(p.Name))
	body.WriteString(": ")
	body.WriteString(m.wizardInput.View())
	if m.wizardLoadingSuggs {
		body.WriteString("  Loading...")
	}
	body.WriteString("\n")

	// Error
	if m.wizardError != nil {
		body.WriteString("\n")
		body.WriteString("Error: ")
		body.WriteString(m.wizardError.Error())
		body.WriteString("\n")
	}

	// Suggestions
	if !m.wizardLoadingSuggs && m.wizardError == nil && len(m.wizardSuggestions) > 0 {
		body.WriteString("\n")
		body.WriteString(headerStyle.Render("suggestions:"))
		body.WriteString("\n")
		for i, sug := range m.wizardSuggestions {
			if i == m.wizardSugSelectedIndex {
				body.WriteString(selectedItemStyle.Render("  ▸ " + sug))
			} else {
				body.WriteString(normalItemStyle.Render("    " + sug))
			}
			body.WriteString("\n")
		}
	}

	// Preview
	previewCode := m.renderPreviewForOverlay()
	if previewCode != "" {
		body.WriteString("\n")
		body.WriteString(headerStyle.Render("── Preview ──"))
		body.WriteString("\n")
		body.WriteString(previewCode)
		body.WriteString("\n")
	}

	// Instructions
	body.WriteString("\n")
	body.WriteString(instructionsStyle.Render("Enter/Tab: next  Shift+Tab: back  Esc: cancel  ↑↓: navigate"))

	overlayWidth := m.termWidth - overlayMargin*2
	overlayHeight := m.termHeight - overlayMargin*2
	if overlayWidth < 40 {
		overlayWidth = 40
	}
	if overlayHeight < 10 {
		overlayHeight = 10
	}

	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#5F5FAF")).
		Background(lipgloss.Color("#1E1E1E")).
		Width(overlayWidth).
		Height(overlayHeight).
		Padding(1, 1)

	return overlayStyle.Render(body.String())
}

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
	paneH := m.paneContentHeight(panelHeight)

	visibleListLines := clipLines(listLines, m.finderScrollOffset, innerH)
	listContent := strings.Join(visibleListLines, "\n")
	if listContent == "" {
		listContent = "  No matches found."
	}

	leftScrollbar := renderScrollbar(len(listLines), m.finderScrollOffset, innerH, innerH)

	previewContent := "No strand selected."
	var previewScrollbar string
	if m.mode == WizardMode {
		previewContent = "Wizard active — see overlay."
	} else if len(filtered) > 0 && m.finderSelectedIndex < len(filtered) {
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
		previewScrollbar = renderScrollbar(len(previewLines), m.finderPreviewScroll, innerH, innerH)
	}

	leftStyle := previewBorder
	rightStyle := previewBorder
	if m.mode == FinderMode {
		if m.finderFocusedPanel == 0 {
			leftStyle = focusedBorder
		} else {
			rightStyle = focusedBorder
		}
	}

	leftPaneWidth := leftWidth
	if leftScrollbar != "" {
		leftPaneWidth = leftWidth - 1
	}
	rightPaneWidth := rightWidth
	if previewScrollbar != "" {
		rightPaneWidth = rightWidth - 1
	}

	leftPaneStyle := leftStyle.Width(leftPaneWidth).Height(paneH)
	leftBox := leftPaneStyle.Render(listContent)
	rightPaneStyle := rightStyle.Width(rightPaneWidth).Height(paneH)
	rightBox := rightPaneStyle.Render(previewContent)

	if leftScrollbar != "" {
		scrollbarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Height(paneH)
		leftBox = lipgloss.JoinHorizontal(lipgloss.Top, leftBox, scrollbarStyle.Render(leftScrollbar))
	}
	if previewScrollbar != "" {
		scrollbarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Height(paneH)
		rightBox = lipgloss.JoinHorizontal(lipgloss.Top, rightBox, scrollbarStyle.Render(previewScrollbar))
	}

	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	return joined + "\n"
}

func (m *AppModel) renderBrowserPanels() string {
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

	leftWidth, rightWidth, panelHeight := m.panelDims()
	innerH := m.innerContentHeight(panelHeight)
	paneH := m.paneContentHeight(panelHeight)

	visibleListLines := clipLines(listLines, m.browserScrollOffset, innerH)
	listContent := strings.Join(visibleListLines, "\n")
	if listContent == "" {
		listContent = "  Empty directory."
	}

	leftScrollbar := renderScrollbar(len(listLines), m.browserScrollOffset, innerH, innerH)

	previewContent := "No strand selected."
	var previewScrollbar string
	if len(m.browserItems) > 0 && m.browserSelectedIndex < len(m.browserItems) {
		item := m.browserItems[m.browserSelectedIndex]
		if item.isSnip {
			rendered := m.renderSnippetPreview(item.snip)
			previewLines := strings.Split(rendered, "\n")

			maxPreviewScroll := len(previewLines) - innerH
			if maxPreviewScroll < 0 {
				maxPreviewScroll = 0
			}
			if m.browserPreviewScroll > maxPreviewScroll {
				m.browserPreviewScroll = maxPreviewScroll
			}

			visiblePreviewLines := clipLines(previewLines, m.browserPreviewScroll, innerH)
			previewContent = strings.Join(visiblePreviewLines, "\n")
			previewScrollbar = renderScrollbar(len(previewLines), m.browserPreviewScroll, innerH, innerH)
		}
	}

	leftStyle := previewBorder
	rightStyle := previewBorder
	if m.browserFocusedPanel == 0 {
		leftStyle = focusedBorder
	} else {
		rightStyle = focusedBorder
	}

	leftPaneWidth := leftWidth
	if leftScrollbar != "" {
		leftPaneWidth = leftWidth - 1
	}
	rightPaneWidth := rightWidth
	if previewScrollbar != "" {
		rightPaneWidth = rightWidth - 1
	}

	leftPaneStyle := leftStyle.Width(leftPaneWidth).Height(paneH)
	leftBox := leftPaneStyle.Render(listContent)
	rightPaneStyle := rightStyle.Width(rightPaneWidth).Height(paneH)
	rightBox := rightPaneStyle.Render(previewContent)

	if leftScrollbar != "" {
		scrollbarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Height(paneH)
		leftBox = lipgloss.JoinHorizontal(lipgloss.Top, leftBox, scrollbarStyle.Render(leftScrollbar))
	}
	if previewScrollbar != "" {
		scrollbarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Height(paneH)
		rightBox = lipgloss.JoinHorizontal(lipgloss.Top, rightBox, scrollbarStyle.Render(previewScrollbar))
	}

	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	return joined + "\n"
}

func (m *AppModel) renderTagsPanels() string {
	leftWidth, rightWidth, panelHeight := m.panelDims()
	innerH := m.innerContentHeight(panelHeight)
	paneH := m.paneContentHeight(panelHeight)

	var listLines []string
	var previewContent string
	var listContent string
	var listScrollbar string
	var previewScrollbar string

	if m.inTagSnippets {
		for i, snip := range m.tagSnippets {
			label := fmt.Sprintf("  %s (%s)", snip.Name, snip.RelativePath)
			if i == m.tagSnippetsSelectedIndex {
				listLines = append(listLines, selectedItemStyle.Render("▸"+label[1:]))
			} else {
				listLines = append(listLines, normalItemStyle.Render(label))
			}
		}

		visibleListLines := clipLines(listLines, m.tagSnippetsScrollOffset, innerH)
		listContent = strings.Join(visibleListLines, "\n")
		if listContent == "" {
			listContent = "  No strands in tag."
		}

		listScrollbar = renderScrollbar(len(listLines), m.tagSnippetsScrollOffset, innerH, innerH)

		previewContent = "No strand selected."
		if len(m.tagSnippets) > 0 && m.tagSnippetsSelectedIndex < len(m.tagSnippets) {
			rendered := m.renderSnippetPreview(m.tagSnippets[m.tagSnippetsSelectedIndex])
			previewLines := strings.Split(rendered, "\n")

			maxPreviewScroll := len(previewLines) - innerH
			if maxPreviewScroll < 0 {
				maxPreviewScroll = 0
			}
			if m.tagSnippetsPreviewScroll > maxPreviewScroll {
				m.tagSnippetsPreviewScroll = maxPreviewScroll
			}

			visiblePreviewLines := clipLines(previewLines, m.tagSnippetsPreviewScroll, innerH)
			previewContent = strings.Join(visiblePreviewLines, "\n")
			previewScrollbar = renderScrollbar(len(previewLines), m.tagSnippetsPreviewScroll, innerH, innerH)
		}
	} else {
		for i, tag := range m.tags {
			count := m.tagCounts[tag]
			label := fmt.Sprintf("  #%s (%d)", tag, count)
			if i == m.tagsSelectedIndex {
				listLines = append(listLines, selectedItemStyle.Render("▸"+label[1:]))
			} else {
				listLines = append(listLines, normalItemStyle.Render(label))
			}
		}

		visibleListLines := clipLines(listLines, m.tagsScrollOffset, innerH)
		listContent = strings.Join(visibleListLines, "\n")
		if listContent == "" {
			listContent = "  No tags found."
		}

		listScrollbar = renderScrollbar(len(listLines), m.tagsScrollOffset, innerH, innerH)

		previewContent = "Select a tag to see strands."
		if len(m.tags) > 0 && m.tagsSelectedIndex < len(m.tags) {
			tag := m.tags[m.tagsSelectedIndex]
			count := m.tagCounts[tag]
			previewContent = fmt.Sprintf("# %s\n\n**%d strands**\n\nSelect this tag to browse its strands.", tag, count)
			rendered, _ := mdRenderer.Render(previewContent)
			previewLines := strings.Split(rendered, "\n")

			maxPreviewScroll := len(previewLines) - innerH
			if maxPreviewScroll < 0 {
				maxPreviewScroll = 0
			}
			if m.tagsPreviewScroll > maxPreviewScroll {
				m.tagsPreviewScroll = maxPreviewScroll
			}

			visiblePreviewLines := clipLines(previewLines, m.tagsPreviewScroll, innerH)
			previewContent = strings.Join(visiblePreviewLines, "\n")
			previewScrollbar = renderScrollbar(len(previewLines), m.tagsPreviewScroll, innerH, innerH)
		}
	}

	leftStyle := previewBorder
	rightStyle := previewBorder
	if m.inTagSnippets {
		if m.tagSnippetsFocusedPanel == 0 {
			leftStyle = focusedBorder
		} else {
			rightStyle = focusedBorder
		}
	} else {
		if m.tagsFocusedPanel == 0 {
			leftStyle = focusedBorder
		} else {
			rightStyle = focusedBorder
		}
	}

	leftPaneWidth := leftWidth
	if listScrollbar != "" {
		leftPaneWidth = leftWidth - 1
	}
	rightPaneWidth := rightWidth
	if previewScrollbar != "" {
		rightPaneWidth = rightWidth - 1
	}

	leftPaneStyle := leftStyle.Width(leftPaneWidth).Height(paneH)
	leftBox := leftPaneStyle.Render(listContent)
	rightPaneStyle := rightStyle.Width(rightPaneWidth).Height(paneH)
	rightBox := rightPaneStyle.Render(previewContent)

	if listScrollbar != "" {
		scrollbarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Height(paneH)
		leftBox = lipgloss.JoinHorizontal(lipgloss.Top, leftBox, scrollbarStyle.Render(listScrollbar))
	}
	if previewScrollbar != "" {
		scrollbarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Height(paneH)
		rightBox = lipgloss.JoinHorizontal(lipgloss.Top, rightBox, scrollbarStyle.Render(previewScrollbar))
	}

	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	return joined + "\n"
}

func (m *AppModel) headerHeight() int {
	switch m.mode {
	case FinderMode, WizardMode:
		return 4
	case CommandPaletteMode:
		switch m.commandPalettePrevMode {
		case FinderMode, WizardMode:
			return 4
		}
		return 4
	default:
		return 1
	}
}

// overlayOnView computes the starting position for the popup and delegates to renderWithFloatingPane's overlay logic.
func (m *AppModel) overlayOnView(baseView string, popupContent string) string {
	baseLines := strings.Split(baseView, "\n")
	popupLines := strings.Split(popupContent, "\n")

	popupWidth := 0
	for _, l := range popupLines {
		if w := ansi.StringWidth(l); w > popupWidth {
			popupWidth = w
		}
	}
	popupHeight := len(popupLines)

	if popupWidth <= 0 || popupHeight <= 0 {
		return baseView
	}

	startX := overlayMargin
	startY := overlayMargin
	if startX < 0 {
		startX = 0
	}
	if startY < overlayMargin {
		startY = overlayMargin
	}
	if startY+popupHeight > m.termHeight {
		startY = m.termHeight - popupHeight
		if startY < overlayMargin {
			startY = overlayMargin
		}
	}

	var result strings.Builder
	for i := 0; i < m.termHeight; i++ {
		if i >= startY && i < startY+popupHeight && i-startY < len(popupLines) {
			baseLine := ""
			if i < len(baseLines) {
				baseLine = baseLines[i]
			}

			contentLine := popupLines[i-startY]
			contentWidth := ansi.StringWidth(contentLine)

			leftPart := ansi.Cut(baseLine, 0, startX)
			result.WriteString(leftPart)

			result.WriteString(contentLine)

			rightPart := ansi.Cut(baseLine, startX+contentWidth, m.termWidth)
			result.WriteString(rightPart)

			result.WriteString("\n")
		} else if i < len(baseLines) {
			result.WriteString(baseLines[i])
			result.WriteString("\n")
		} else {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m *AppModel) renderWithFloatingPane(baseView string) string {
	const minPaneHeight = 8
	const minPaneWidth = 30

	if m.termHeight < minPaneHeight+4 || m.termWidth < minPaneWidth+4 {
		return baseView
	}

	baseLines := strings.Split(baseView, "\n")

	for len(baseLines) < m.termHeight {
		baseLines = append(baseLines, "")
	}

	floatingWidth := m.floatingPane.width
	floatingHeight := m.floatingPane.height

	if floatingWidth < minPaneWidth {
		floatingWidth = minPaneWidth
	}
	if floatingHeight < minPaneHeight {
		floatingHeight = minPaneHeight
	}
	if floatingWidth > m.termWidth-4 {
		floatingWidth = m.termWidth - 4
	}
	if floatingHeight > m.termHeight-4 {
		floatingHeight = m.termHeight - 4
	}

	headerRows := m.headerHeight()
	startX := (m.termWidth - floatingWidth) / 2
	availHeight := m.termHeight - headerRows
	startY := headerRows + (availHeight-floatingHeight)/2

	if startX < 0 {
		startX = 0
	}
	if startY < headerRows {
		startY = headerRows
	}
	if startY+floatingHeight > m.termHeight {
		startY = m.termHeight - floatingHeight
		if startY < headerRows {
			startY = headerRows
		}
	}

	floatingPaneStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFD700")).
		Background(lipgloss.Color("#1E1E1E")).
		Width(floatingWidth-2).
		Height(floatingHeight-2).
		Padding(1, 1)

	var contentBuilder strings.Builder
	if m.floatingPane.title != "" {
		contentBuilder.WriteString(titleStyle.Render(m.floatingPane.title) + "\n\n")
	}
	for _, line := range m.floatingPane.content {
		contentBuilder.WriteString(line + "\n")
	}

	renderedContent := floatingPaneStyle.Render(contentBuilder.String())
	contentLines := strings.Split(renderedContent, "\n")

	var result strings.Builder
	for i := 0; i < m.termHeight; i++ {
		if i >= startY && i < startY+floatingHeight && i-startY < len(contentLines) {
			baseLine := ""
			if i < len(baseLines) {
				baseLine = baseLines[i]
			}

			contentLine := contentLines[i-startY]
			contentWidth := ansi.StringWidth(contentLine)

			leftPart := ansi.Cut(baseLine, 0, startX)
			result.WriteString(leftPart)

			result.WriteString(contentLine)

			rightPart := ansi.Cut(baseLine, startX+contentWidth, m.termWidth)
			result.WriteString(rightPart)

			result.WriteString("\n")
		} else if i < len(baseLines) {
			result.WriteString(baseLines[i])
			result.WriteString("\n")
		} else {
			result.WriteString("\n")
		}
	}

	return result.String()
}
