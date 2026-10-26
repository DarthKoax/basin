package tui

import (
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