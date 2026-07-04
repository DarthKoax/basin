package tui

import (
    "strings"
    "testing"

    "peanutclone/internal/config"
    "peanutclone/internal/domain"
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
