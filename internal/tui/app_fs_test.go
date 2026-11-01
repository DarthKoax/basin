package tui

import (
	"fmt"
	"strings"
	"testing"

	"basin/internal/config"
	"basin/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFS_ToggleFullscreen(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.finderFocusedPanel = 1
	m.termWidth = 120
	m.termHeight = 40

	if m.fullscreenActive {
		t.Fatal("expected fullscreen inactive initially")
	}
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	model, _ := m.Update(key)
	m = model.(*AppModel)
	if !m.fullscreenActive {
		t.Fatal("expected fullscreen active after f")
	}
	model, _ = m.Update(key)
	m = model.(*AppModel)
	if m.fullscreenActive {
		t.Fatal("expected fullscreen inactive after second f")
	}
}

func TestFS_DisabledInWizard(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.mode = WizardMode
	m.wizardSnippet = snip
	m.wizardVarIndex = 0
	m.wizardConfirmed = make(map[string]string)
	m.termWidth = 120
	m.termHeight = 40

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	model, _ := m.Update(key)
	m = model.(*AppModel)
	if m.fullscreenActive {
		t.Fatal("expected fullscreen disabled in wizard")
	}
}

func TestFS_FullFilePreviewToggle(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{Name: "s", RelativePath: "test.md", Body: "echo hi", Language: "sh", SourceFile: "test.md"}
	files := []domain.SnippetFile{{Path: "test.md", RelativePath: "test.md", Snippets: []domain.Snippet{snip}}}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, files)
	m.finderFocusedPanel = 1
	m.termWidth = 120
	m.termHeight = 40

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}
	model, _ := m.Update(key)
	m = model.(*AppModel)
	if !m.fullFilePreviewActive {
		t.Fatal("expected full file preview active")
	}
	view := m.View()
	if !strings.Contains(stripANSI(view), "Full File:") {
		t.Fatalf("expected header, got: %s", view)
	}
	// exit
	key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}
	model, _ = m.Update(key)
	m = model.(*AppModel)
	if m.fullFilePreviewActive {
		t.Fatal("expected full file preview inactive")
	}
}

func TestFS_FullFilePreviewError(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{Name: "s", RelativePath: "test.md", Body: "echo hi", Language: "sh"}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.finderFocusedPanel = 1
	m.termWidth = 120
	m.termHeight = 40

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}
	model, _ := m.Update(key)
	m = model.(*AppModel)
	view := m.View()
	if !strings.Contains(stripANSI(view), "Error: Could not load file content") {
		t.Fatalf("expected error, got: %s", view)
	}
}

func TestFS_FullscreenViewFooter(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 40
	m.fullscreenActive = true
	view := m.View()
	if !strings.Contains(stripANSI(view), "f/Esc/q: exit fullscreen  ↑↓/jk: scroll  Ctrl+U/D: page scroll") {
		t.Fatalf("expected minimal footer, got: %s", view)
	}
}

// TestFS_FullscreenRetainsHeader verifies that fullscreen mode still renders the
// application header so the view stays within the pane structure.
func TestFS_FullscreenRetainsHeader(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 40
	m.fullscreenActive = true
	view := stripANSI(m.View())
	if !strings.Contains(view, "BASIN") {
		t.Fatalf("expected fullscreen view to contain header 'BASIN', got:\n%s", view)
	}
}

// TestFS_FullscreenPaneHasBorder verifies the fullscreen preview is wrapped in
// a bordered pane (left border character │ should be present).
func TestFS_FullscreenPaneHasBorder(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 40
	m.fullscreenActive = true
	view := stripANSI(m.View())
	if !strings.Contains(view, "│") {
		t.Fatalf("expected fullscreen view to have bordered pane with │, got:\n%s", view)
	}
}

// TestFS_FullscreenPaneFillsWidth verifies that the fullscreen pane fills the
// full terminal width. Lines that contain the left border should also extend
// to the full terminal width (right border or scrollbar at the right edge).
func TestFS_FullscreenPaneFillsWidth(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 40
	m.fullscreenActive = true
	view := stripANSI(m.View())
	lines := strings.Split(view, "\n")
	// Find a line that has the left border (content line inside the pane).
	foundContentLine := false
	for _, line := range lines {
		if strings.HasPrefix(line, "│") {
			foundContentLine = true
			// The line should be close to the terminal width (within 2 chars
			// to account for trailing whitespace differences).
			lineLen := len([]rune(line))
			if lineLen < m.termWidth-2 {
				t.Fatalf("expected content line to fill terminal width (%d), got %d: %q", m.termWidth, lineLen, line)
			}
			break
		}
	}
	if !foundContentLine {
		t.Fatalf("expected at least one content line starting with │, got:\n%s", view)
	}
}

// TestFS_FullscreenScrollbarAtRightEdge verifies that when scrolling is needed,
// the scrollbar characters (█ or ░) appear at the rightmost column of the
// terminal width.
func TestFS_FullscreenScrollbarAtRightEdge(t *testing.T) {
	cfg := config.DefaultConfig()
	// Create a snippet with many lines to force scrolling.
	var body strings.Builder
	for i := 0; i < 60; i++ {
		body.WriteString(fmt.Sprintf("echo line %d\n", i))
	}
	snip := domain.Snippet{Name: "long", RelativePath: "test.md", Body: body.String(), Language: "sh"}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 30
	m.finderFocusedPanel = 1
	m.fullscreenActive = true

	view := stripANSI(m.View())
	lines := strings.Split(view, "\n")
	foundScrollbar := false
	for _, line := range lines {
		runes := []rune(line)
		if len(runes) == 0 {
			continue
		}
		// Check if the rightmost non-space character is a scrollbar character.
		lastChar := runes[len(runes)-1]
		if lastChar == '█' || lastChar == '░' {
			foundScrollbar = true
			// The scrollbar char should be at the rightmost position (termWidth column).
			// Allow for trailing whitespace tolerance.
			if len(runes) < m.termWidth-1 {
				t.Fatalf("scrollbar char at column %d, expected near %d", len(runes), m.termWidth)
			}
			break
		}
	}
	if !foundScrollbar {
		t.Fatalf("expected scrollbar at right edge, got:\n%s", view)
	}
}

func TestFS_FullFileAllStrands(t *testing.T) {
	cfg := config.DefaultConfig()
	snip1 := domain.Snippet{Name: "first", RelativePath: "test.md", Body: "echo 1", Language: "sh"}
	snip2 := domain.Snippet{Name: "second", RelativePath: "test.md", Body: "echo 2", Language: "sh"}
	files := []domain.SnippetFile{{Path: "test.md", RelativePath: "test.md", Frontmatter: domain.Frontmatter{Name: "MyFile", Tags: []string{"a"}, Description: "desc"}, Snippets: []domain.Snippet{snip1, snip2}}}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip1, snip2}, files)
	// set SourceFile
	m.allSnippets[0].SourceFile = "test.md"
	m.allSnippets[1].SourceFile = "test.md"
	m.snippetFiles = files
	m.finderFocusedPanel = 1
	m.termWidth = 120
	m.termHeight = 40

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}
	model, _ := m.Update(key)
	m = model.(*AppModel)
	view := stripANSI(m.View())
	if !strings.Contains(view, "first") || !strings.Contains(view, "second") {
		t.Fatalf("expected both strands, got: %s", view)
	}
	if !strings.Contains(view, "MyFile") {
		t.Fatalf("expected frontmatter name, got: %s", view)
	}
}

func TestFS_ScrollKeys(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 40
	m.fullscreenActive = true
	m.finderPreviewScroll = 2

	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	model, _ := m.Update(down)
	m = model.(*AppModel)
	if m.finderPreviewScroll != 3 {
		t.Fatalf("expected scroll 3, got %d", m.finderPreviewScroll)
	}
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	model, _ = m.Update(up)
	m = model.(*AppModel)
	if m.finderPreviewScroll != 2 {
		t.Fatalf("expected scroll 2, got %d", m.finderPreviewScroll)
	}
}

func TestFS_FullFileScroll(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{Name: "s", RelativePath: "test.md", Body: "echo hi", Language: "sh", SourceFile: "test.md"}
	files := []domain.SnippetFile{{Path: "test.md", RelativePath: "test.md", Snippets: []domain.Snippet{snip}}}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, files)
	m.termWidth = 120
	m.termHeight = 40
	m.fullFilePreviewActive = true
	m.loadFullFileContent()

	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	model, _ := m.Update(down)
	m = model.(*AppModel)
	if m.fullFilePreviewScroll != 1 {
		t.Fatalf("expected scroll 1, got %d", m.fullFilePreviewScroll)
	}
}

func TestFS_MouseScroll(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 40
	m.fullscreenActive = true
	m.finderPreviewScroll = 2

	msg := tea.MouseMsg{Button: tea.MouseButtonWheelDown}
	model, _ := m.handleMouse(msg)
	m = model.(*AppModel)
	if m.finderPreviewScroll != 3 {
		t.Fatalf("expected scroll 3, got %d", m.finderPreviewScroll)
	}
}

func TestFS_ExitFromFullscreenFullFile(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{Name: "s", RelativePath: "test.md", Body: "echo hi", Language: "sh", SourceFile: "test.md"}
	files := []domain.SnippetFile{{Path: "test.md", RelativePath: "test.md", Snippets: []domain.Snippet{snip}}}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, files)
	m.termWidth = 120
	m.termHeight = 40
	m.fullFilePreviewActive = true
	m.fullscreenActive = true
	m.loadFullFileContent()

	esc := tea.KeyMsg{Type: tea.KeyEsc}
	model, _ := m.Update(esc)
	m = model.(*AppModel)
	if m.fullFilePreviewActive || m.fullscreenActive {
		t.Fatal("expected both exited")
	}
}

func TestFS_FullscreenToggleInFullFile(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := domain.Snippet{Name: "s", RelativePath: "test.md", Body: "echo hi", Language: "sh", SourceFile: "test.md"}
	files := []domain.SnippetFile{{Path: "test.md", RelativePath: "test.md", Snippets: []domain.Snippet{snip}}}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, files)
	m.termWidth = 120
	m.termHeight = 40
	m.fullFilePreviewActive = true
	m.loadFullFileContent()

	f := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	model, _ := m.Update(f)
	m = model.(*AppModel)
	if !m.fullscreenActive {
		t.Fatal("expected fullscreen active")
	}
	if !m.fullFilePreviewActive {
		t.Fatal("expected still full file preview")
	}
}

func TestFS_SearchInputReceivesF(t *testing.T) {
	cfg := config.DefaultConfig()
	snip := newTestSnippet()
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	m.termWidth = 120
	m.termHeight = 40
	// search input focused by default (finderFocusedPanel == 0)

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	model, _ := m.Update(key)
	m = model.(*AppModel)
	if m.fullscreenActive {
		t.Fatal("expected fullscreen inactive when typing in search")
	}
	if !strings.Contains(m.finderQuery.Value(), "f") {
		t.Fatalf("expected search query to contain 'f', got %q", m.finderQuery.Value())
	}
}

// TestFS_SmallWindowScrollbarAlignment verifies that on small window sizes,
// the scrollbar characters are properly aligned with the content area
// (not overlapping with the pane's top border).
func TestFS_SmallWindowScrollbarAlignment(t *testing.T) {
	cfg := config.DefaultConfig()
	// Create a snippet with many lines to force scrolling
	var body strings.Builder
	for i := 0; i < 50; i++ {
		body.WriteString(fmt.Sprintf("echo line %d\n", i))
	}
	snip := domain.Snippet{Name: "long", RelativePath: "test.md", Body: body.String(), Language: "sh"}
	m := NewAppModel(cfg, nil, []domain.Snippet{snip}, nil)
	
	// Use a small terminal size (80x20)
	m.termWidth = 80
	m.termHeight = 20
	m.finderFocusedPanel = 1
	m.fullscreenActive = true

	view := stripANSI(m.View())
	lines := strings.Split(view, "\n")
	
	// Find a line that contains scrollbar characters
	scrollbarFound := false
	for _, line := range lines {
		runes := []rune(line)
		for _, r := range runes {
			if r == '█' || r == '░' {
				scrollbarFound = true
				// Verify the scrollbar is not on the same line as the top border
				// Top border lines should contain '┌', '┐', or '─'
				if strings.ContainsRune(line, '┌') || strings.ContainsRune(line, '┐') {
					t.Fatalf("scrollbar character found on top border line: %q", line)
				}
				break
			}
		}
		if scrollbarFound {
			break
		}
	}
	
	if !scrollbarFound {
		t.Fatal("expected scrollbar to be present in small window, but none found")
	}
}
