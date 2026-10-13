package tui

import (
	"regexp"
	"strings"
	"testing"

	"basin/internal/config"
	"basin/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func viewContains(view, needle string) bool {
	return strings.Contains(stripANSI(view), needle)
}

func viewHasSideBySide(view string) bool {
	clean := stripANSI(view)
	return strings.Contains(clean, "┐┌") || strings.Contains(clean, "┐ ┌")
}

func newTestSnippet() domain.Snippet {
	return domain.Snippet{
		Name:         "test-snippet",
		RelativePath: "test.md",
		Description:  "A test snippet",
		Body:         "echo <$var>",
		Language:     "sh",
		Placeholders: []domain.Placeholder{{Name: "var", IsCommand: false}},
	}
}

func newTestModelWithWizard(cfg *config.Config, termWidth, termHeight int) *AppModel {
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = termWidth
	m.termHeight = termHeight
	return m
}

// Test that entering wizard mode renders the wizard overlay.
func TestWizardOverlayDisplaysBackground(t *testing.T) {
	cfg := config.DefaultConfig()
	m := newTestModelWithWizard(cfg, 120, 40)
	view := m.View()
	if !viewContains(view, "var:") {
		t.Fatalf("expected wizard overlay to contain variable prompt, got: %s", view)
	}
	if !viewContains(view, "┌") && !viewContains(view, "│") {
		t.Fatalf("expected borders in view, got: %s", view)
	}
}

// Test that wizard mode renders snippet list on left and preview on right.
func TestWizardPanelLayout(t *testing.T) {
	cfg := config.DefaultConfig()
	m := newTestModelWithWizard(cfg, 160, 40)
	view := m.View()

	if !viewContains(view, "test-snippet") {
		t.Fatal("expected preview to contain snippet name")
	}
	if !viewContains(view, "var:") {
		t.Fatal("expected wizard bar to be visible in header")
	}
	if !viewContains(view, "[1/1]") {
		t.Fatal("expected wizard progress in bar")
	}
	if !viewHasSideBySide(view) {
		t.Fatalf("expected snippet list and preview to render side by side\n%s", view)
	}
}

// Test that wizard overlay shows progress and variable name.
func TestWizardOverlayContent(t *testing.T) {
	cfg := config.DefaultConfig()
	m := newTestModelWithWizard(cfg, 120, 50)
	view := m.View()

	if !viewContains(view, "[1/1]") {
		t.Fatal("expected wizard progress in overlay")
	}
	if !viewContains(view, "test-snippet > var") {
		t.Fatal("expected wizard overlay to contain snippet name and variable")
	}
}

// Test that custom width percentages affect the panel sizing in wizard mode.
func TestWizardResizableWidth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Panel.WidthPercent = 75
	m := newTestModelWithWizard(cfg, 160, 40)
	view := m.View()

	if !viewContains(view, "var:") {
		t.Fatal("expected wizard bar content with custom width")
	}
	if !viewContains(view, "[1/1]") {
		t.Fatal("expected overlay progress with custom width")
	}
}

// Test that the wizard overlay instructions are visible.
func TestWizardShowsFinderHeader(t *testing.T) {
	cfg := config.DefaultConfig()
	m := newTestModelWithWizard(cfg, 160, 40)
	view := m.View()

	if !strings.Contains(view, "BASIN") {
		t.Fatal("expected header title in wizard view")
	}
	if !strings.Contains(view, "Fuzzyfinder") {
		t.Fatal("expected Fuzzyfinder mode label in wizard header")
	}
	if !strings.Contains(view, "Enter/Tab: next") || !strings.Contains(view, "Esc: cancel") {
		t.Fatal("expected wizard instructions in wizard view")
	}
}

func TestWizardSelectPlaceholder_SuggestionsRendered(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{
		Name:         "HTTP request",
		RelativePath: "http.md",
		Description:  "Send an HTTP request.",
		Body:         "curl -X <$METHOD> $URL",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST", "PUT", "DELETE"},
			},
			{
				Name: "URL",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, nil, snippets, files)
	m.history = nil
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 160
	m.termHeight = 40

	// Trigger wizard setup to load suggestions
	cmd := m.setupWizardForCurrentVar()
	if cmd != nil {
		t.Fatal("select placeholder should not return a background cmd")
	}

	if len(m.wizardSuggestions) == 0 {
		t.Fatalf("expected wizard suggestions to be populated, got nil/empty")
	}

	expectedOpts := []string{"GET", "POST", "PUT", "DELETE"}
	if len(m.wizardSuggestions) != len(expectedOpts) {
		t.Errorf("expected %d suggestions, got %d: %v", len(expectedOpts), len(m.wizardSuggestions), m.wizardSuggestions)
	}

	for i, opt := range expectedOpts {
		if m.wizardSuggestions[i] != opt {
			t.Errorf("suggestions[%d] = %q, expected %q", i, m.wizardSuggestions[i], opt)
		}
	}

	// Default value should be the first option
	if m.wizardInput.Value() != "GET" {
		t.Errorf("expected input pre-filled with 'GET', got %q", m.wizardInput.Value())
	}

	view := m.View()
	if !strings.Contains(view, "suggestions:") {
		t.Fatal("expected wizard view to contain suggestions label")
	}
	for _, opt := range expectedOpts {
		if !strings.Contains(view, opt) {
			t.Errorf("expected suggestion %q in wizard view", opt)
		}
	}
}

func TestWizardSelect_NavigateSuggestions(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{
		Name:         "HTTP request",
		RelativePath: "http.md",
		Description:  "Send an HTTP request.",
		Body:         "curl -X <$METHOD> $URL",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST"},
			},
			{
				Name: "URL",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, nil, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 160
	m.termHeight = 40

	// Load suggestions
	m.setupWizardForCurrentVar()

	// Simulate pressing down to select POST
	downKey := tea.KeyMsg{Type: tea.KeyDown}

	model, _ := m.Update(downKey)
	m = model.(*AppModel)

	if m.wizardSugSelectedIndex != 1 {
		t.Errorf("expected suggestion index 1 (POST), got %d", m.wizardSugSelectedIndex)
	}
	if m.wizardInput.Value() != "POST" {
		t.Errorf("expected input to be 'POST', got %q", m.wizardInput.Value())
	}

	upKey := tea.KeyMsg{Type: tea.KeyUp}

	model, _ = m.Update(upKey)
	m = model.(*AppModel)

	if m.wizardSugSelectedIndex != 0 {
		t.Errorf("expected suggestion index 0 (GET), got %d", m.wizardSugSelectedIndex)
	}
	if m.wizardInput.Value() != "GET" {
		t.Errorf("expected input to be 'GET', got %q", m.wizardInput.Value())
	}
}

func TestWizardSelect_ConfirmAndAdvance(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{
		Name:         "HTTP request",
		RelativePath: "http.md",
		Description:  "Send an HTTP request.",
		Body:         "curl -X <$METHOD> $URL",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST"},
			},
			{
				Name: "URL",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, nil, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 160
	m.termHeight = 40

	// Load suggestions for first variable (METHOD)
	m.setupWizardForCurrentVar()

	downKey := tea.KeyMsg{Type: tea.KeyDown}

	model, _ := m.Update(downKey)
	m = model.(*AppModel)

	if m.wizardInput.Value() != "POST" {
		t.Fatalf("expected input 'POST', got %q", m.wizardInput.Value())
	}

	// Press Enter to confirm selection
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	model, _ = m.Update(enter)
	m = model.(*AppModel)

	if m.wizardVarIndex != 1 {
		t.Errorf("expected var index to advance to 1, got %d", m.wizardVarIndex)
	}

	if m.wizardConfirmed["METHOD"] != "POST" {
		t.Errorf("expected METHOD confirmed as 'POST', got %q", m.wizardConfirmed["METHOD"])
	}

	currentP := m.wizardSnippet.Placeholders[m.wizardVarIndex]
	if currentP.Name != "URL" {
		t.Errorf("expected current variable to be URL, got %q", currentP.Name)
	}
}

func TestWizardSelect_FullRenderConfirmCopiesSnippet(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{
		Name:         "HTTP request",
		RelativePath: "http.md",
		Description:  "Send an HTTP request.",
		Body:         "curl -X <$METHOD> <$URL>",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST"},
			},
			{
				Name: "URL",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, nil, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 160
	m.termHeight = 40

	// First variable: select POST (press down once, then enter)
	m.setupWizardForCurrentVar()

	down := tea.KeyMsg{Type: tea.KeyDown}

	model, _ := m.Update(down)
	m = model.(*AppModel)

	enter := tea.KeyMsg{Type: tea.KeyEnter}

	model, _ = m.Update(enter)
	m = model.(*AppModel)

	// Second variable: type a URL value then enter
	typingMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://example.com")}

	model, _ = m.Update(typingMsg)
	m = model.(*AppModel)

	if !strings.Contains(m.wizardInput.Value(), "example.com") {
		t.Fatalf("expected URL input to contain 'example.com', got %q", m.wizardInput.Value())
	}

	// Confirm second variable to finalize snippet
	model, _ = m.Update(enter)
	m = model.(*AppModel)

	if !m.Done {
		t.Fatal("expected wizard to be done after confirming all variables")
	}

	if !strings.Contains(m.FinalSnippet, "POST") {
		t.Errorf("expected final snippet to contain POST, got: %s", m.FinalSnippet)
	}

	if !strings.Contains(m.FinalSnippet, "https://example.com") {
		t.Errorf("expected final snippet to contain URL, got: %s", m.FinalSnippet)
	}
}

func TestFloatingPane_ShowAndHide(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	if m.floatingPane.active {
		t.Fatal("floating pane should not be active initially")
	}

	m.showFloatingPane("Test Pane", []string{"Line 1", "Line 2"}, 40, 10)

	if !m.floatingPane.active {
		t.Fatal("floating pane should be active after showFloatingPane")
	}
	if m.floatingPane.title != "Test Pane" {
		t.Errorf("expected title 'Test Pane', got %q", m.floatingPane.title)
	}
	if len(m.floatingPane.content) != 2 {
		t.Errorf("expected 2 content lines, got %d", len(m.floatingPane.content))
	}

	view := m.View()
	if !viewContains(view, "Test Pane") {
		t.Fatal("expected view to contain floating pane title")
	}
	if !viewContains(view, "Line 1") {
		t.Fatal("expected view to contain floating pane content")
	}

	m.hideFloatingPane()
	if m.floatingPane.active {
		t.Fatal("floating pane should not be active after hideFloatingPane")
	}
}

func TestFloatingPane_DismissWithEsc(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	m.showFloatingPane("Test", []string{"Content"}, 40, 10)

	escKey := tea.KeyMsg{Type: tea.KeyEsc}
	model, _ := m.Update(escKey)
	m = model.(*AppModel)

	if m.floatingPane.active {
		t.Fatal("floating pane should be dismissed after pressing Esc")
	}
}

func TestFloatingPane_CenteredPosition(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 100
	m.termHeight = 50

	m.showFloatingPane("Centered", []string{"Test"}, 40, 10)

	view := m.View()
	lines := strings.Split(stripANSI(view), "\n")

	foundTitle := false
	for _, line := range lines {
		if strings.Contains(line, "Centered") {
			foundTitle = true
			break
		}
	}

	if !foundTitle {
		t.Fatal("expected to find floating pane title in view")
	}
}

func TestFloatingPane_SmallTerminal(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 50
	m.termHeight = 12

	m.showFloatingPane("Test", []string{"Content"}, 40, 10)

	view := m.View()

	if !viewContains(view, "Test") {
		t.Fatal("expected floating pane to still render in small terminal")
	}
}

func TestFloatingPane_TooSmallTerminal(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 10
	m.termHeight = 4

	m.showFloatingPane("Test", []string{"Content"}, 40, 10)

	view := m.View()

	if viewContains(view, "Test") {
		t.Fatal("expected floating pane NOT to render when terminal is too small")
	}
}

func TestPaneCollapse_LeftCollapse(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	if m.leftCollapsed {
		t.Fatal("left pane should not be collapsed initially")
	}

	leftBefore, _, _ := m.panelDims()

	m.leftCollapsed = true
	leftAfter, _, _ := m.panelDims()

	if leftAfter >= leftBefore {
		t.Errorf("expected left pane to shrink after collapse, before=%d after=%d", leftBefore, leftAfter)
	}
	if leftAfter > 5 {
		t.Errorf("expected collapsed left pane to be very narrow, got %d", leftAfter)
	}
}

func TestPaneCollapse_RightCollapse(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	if m.rightCollapsed {
		t.Fatal("right pane should not be collapsed initially")
	}

	_, rightBefore, _ := m.panelDims()

	m.rightCollapsed = true
	_, rightAfter, _ := m.panelDims()

	if rightAfter >= rightBefore {
		t.Errorf("expected right pane to shrink after collapse, before=%d after=%d", rightBefore, rightAfter)
	}
	if rightAfter > 5 {
		t.Errorf("expected collapsed right pane to be very narrow, got %d", rightAfter)
	}
}

func TestPaneResize_Keybindings(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	initialPct := m.runtimeWidthPct

	leftBracket := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}
	model, _ := m.Update(leftBracket)
	m = model.(*AppModel)

	if m.runtimeWidthPct >= initialPct {
		t.Errorf("expected width percent to decrease after '[', got %d (was %d)", m.runtimeWidthPct, initialPct)
	}

	rightBracket := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}
	model, _ = m.Update(rightBracket)
	m = model.(*AppModel)

	if m.runtimeWidthPct != initialPct {
		t.Errorf("expected width percent to return to initial after ']', got %d (expected %d)", m.runtimeWidthPct, initialPct)
	}
}

func TestPaneCollapse_Keybindings(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	leftBrace := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}}
	model, _ := m.Update(leftBrace)
	m = model.(*AppModel)

	if !m.leftCollapsed {
		t.Fatal("expected left pane to collapse after '{' key")
	}

	model, _ = m.Update(leftBrace)
	m = model.(*AppModel)

	if m.leftCollapsed {
		t.Fatal("expected left pane to uncollapse after second '{' key")
	}

	rightBrace := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}}
	model, _ = m.Update(rightBrace)
	m = model.(*AppModel)

	if !m.rightCollapsed {
		t.Fatal("expected right pane to collapse after '}' key")
	}
}

func TestRuntimeWidthPct_Clamped(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	m.runtimeWidthPct = 2
	leftBracket := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}
	model, _ := m.Update(leftBracket)
	m = model.(*AppModel)

	if m.runtimeWidthPct < 5 {
		t.Errorf("expected runtimeWidthPct to be clamped to minimum 5, got %d", m.runtimeWidthPct)
	}

	m.runtimeWidthPct = 89
	rightBracket := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}
	model, _ = m.Update(rightBracket)
	m = model.(*AppModel)

	if m.runtimeWidthPct > 90 {
		t.Errorf("expected runtimeWidthPct to be clamped to maximum 90, got %d", m.runtimeWidthPct)
	}
}

func TestOutputConfig_DisableClipboard(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Output.Clipboard = false
	snip := domain.Snippet{
		Name:         "no-clip",
		RelativePath: "test.md",
		Body:         "echo hello",
		Language:     "sh",
	}
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, nil, snippets, files)
	m.termWidth = 120
	m.termHeight = 40

	cmd := m.finalizeSnippet("echo hello")
	if cmd == nil {
		t.Fatal("expected finalizeSnippet to return a quit command")
	}
	if !m.Done {
		t.Fatal("expected Done to be true")
	}
	if m.FinalSnippet != "echo hello" {
		t.Errorf("expected FinalSnippet to be 'echo hello', got %q", m.FinalSnippet)
	}
}

func TestAutoCopyOnSelect_DisabledByDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.Behavior.AutoCopyOnSelect {
		t.Fatal("expected AutoCopyOnSelect to be disabled by default")
	}
}

func TestHelpOverlay_FinderMode(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{
		Name:         "test",
		RelativePath: "test.md",
		Body:         "echo test",
		Language:     "sh",
	}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.mode = FinderMode
	m.termWidth = 120
	m.termHeight = 40

	modeName, bindings := m.getModeKeybindings()
	if modeName != "FINDER" {
		t.Errorf("expected mode name FINDER, got %q", modeName)
	}
	if len(bindings) == 0 {
		t.Fatal("expected bindings to be non-empty")
	}
	
	foundCtrlP := false
	for _, b := range bindings {
		if b[0] == "Ctrl+P" {
			foundCtrlP = true
			break
		}
	}
	if !foundCtrlP {
		t.Error("expected to find Ctrl+P in Finder mode bindings")
	}
}

func TestHelpOverlay_WizardMode(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)

	modeName, bindings := m.getModeKeybindings()
	if modeName != "WIZARD" {
		t.Errorf("expected mode name WIZARD, got %q", modeName)
	}
	if len(bindings) == 0 {
		t.Fatal("expected bindings to be non-empty")
	}
	
	foundTab := false
	foundEsc := false
	for _, b := range bindings {
		if b[0] == "Tab" {
			foundTab = true
		}
		if b[0] == "Esc" {
			foundEsc = true
		}
	}
	if !foundTab {
		t.Error("expected to find Tab in Wizard mode bindings")
	}
	if !foundEsc {
		t.Error("expected to find Esc in Wizard mode bindings")
	}
}

func TestHelpOverlay_CommandPaletteMode(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewAppModel(cfg, nil, nil, nil)
	m.mode = CommandPaletteMode
	m.commandPalettePrevMode = FinderMode

	modeName, bindings := m.getModeKeybindings()
	if modeName != "COMMAND PALETTE" {
		t.Errorf("expected mode name COMMAND PALETTE, got %q", modeName)
	}
	if len(bindings) == 0 {
		t.Fatal("expected bindings to be non-empty")
	}
}

func TestHelpOverlay_TagsMode_InTagSnippets(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewAppModel(cfg, nil, nil, nil)
	m.mode = TagsMode
	m.inTagSnippets = true

	modeName, bindings := m.getModeKeybindings()
	if modeName != "TAGS > STRANDS" {
		t.Errorf("expected mode name TAGS > STRANDS, got %q", modeName)
	}
	if len(bindings) == 0 {
		t.Fatal("expected bindings to be non-empty")
	}
}

func TestHelpOverlay_Keybinding(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{
		Name:         "test",
		RelativePath: "test.md",
		Body:         "echo test",
		Language:     "sh",
	}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.mode = FinderMode
	m.termWidth = 120
	m.termHeight = 40

	if m.floatingPane.active {
		t.Fatal("expected floating pane to be inactive initially")
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	if !m.floatingPane.active {
		t.Fatal("expected floating pane to be active after pressing ?")
	}

	if m.floatingPane.title != "Help" {
		t.Errorf("expected floating pane title to be 'Help', got %q", m.floatingPane.title)
	}

	if len(m.floatingPane.content) == 0 {
		t.Fatal("expected floating pane content to be non-empty")
	}

	hasKeyboardShortcuts := false
	for _, line := range m.floatingPane.content {
		if strings.Contains(line, "Keyboard Shortcuts") {
			hasKeyboardShortcuts = true
			break
		}
	}
	if !hasKeyboardShortcuts {
		t.Error("expected floating pane content to contain 'Keyboard Shortcuts'")
	}
}
