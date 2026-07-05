package tui

import (
	"strings"
	"testing"

	"peanutclone/internal/config"
	"peanutclone/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

// Test that entering wizard mode renders the overlay while keeping background panels.
func TestWizardOverlayDisplaysBackground(t *testing.T) {
	cfg := &config.Config{}
	snip := domain.Snippet{
		Name:         "test-snippet",
		RelativePath: "test.md",
		Description:  "A test snippet",
		Body:         "echo <@var>",
		Language:     "sh",
		Placeholders: []domain.Placeholder{{Name: "var", IsCommand: false}},
	}
	snippets := []domain.Snippet{snip}
	files := []domain.SnippetFile{}
	m := NewAppModel(cfg, snippets, files)
	// Simulate entering wizard mode directly.
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	// Set a reasonable terminal size.
	m.termWidth = 100
	m.termHeight = 40
	// Render view.
	view := m.View()
	if !strings.Contains(view, "test-snippet") {
		t.Fatalf("expected background to contain snippet name, got: %s", view)
	}
	if !strings.Contains(view, "Variable: var") {
		t.Fatalf("expected wizard overlay to contain variable prompt, got: %s", view)
	}
	// Ensure overlay markers are present (border characters)
	if !strings.Contains(view, "┌") && !strings.Contains(view, "│") {
		t.Fatalf("expected overlay borders in view, got: %s", view)
	}
}

func TestWizardSelectPlaceholder_SuggestionsRendered(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{
		Name:         "HTTP request",
		RelativePath: "http.md",
		Description:  "Send an HTTP request.",
		Body:         "curl -X <$METHOD:?GET|POST|PUT|DELETE> $URL",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				Raw:           "<$METHOD:?GET|POST|PUT|DELETE>",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST", "PUT", "DELETE"},
			},
			{
				Name: "URL",
				Raw:  "$URL",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 100
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
	if !strings.Contains(view, "Suggestions:") {
		t.Fatal("expected wizard view to contain Suggestions header")
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
		Body:         "curl -X <$METHOD:?GET|POST> $URL",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				Raw:           "<$METHOD:?GET|POST>",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST"},
			},
			{
				Name: "URL",
				Raw:  "$URL",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 100
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
		Body:         "curl -X <$METHOD:?GET|POST> $URL",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				Raw:           "<$METHOD:?GET|POST>",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST"},
			},
			{
				Name: "URL",
				Raw:  "$URL",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 100
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
		Body:         "curl -X <$METHOD:?GET|POST> <$URL>",
		Language:     "sh",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				Raw:           "<$METHOD:?GET|POST>",
				IsSelect:      true,
				IsDefault:     true,
				DefaultValue:  "GET",
				SelectOptions: []string{"GET", "POST"},
			},
			{
				Name: "URL",
				Raw:  "<$URL>",
			},
		},
	}

	files := []domain.SnippetFile{
		{Path: "http.md", Snippets: []domain.Snippet{snip}},
	}
	snippets := []domain.Snippet{snip}

	m := NewAppModel(cfg, snippets, files)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 100
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
