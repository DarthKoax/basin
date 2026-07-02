package tui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"peanutclone/internal/config"
	"peanutclone/internal/domain"
)

func TestTUI_FuzzyFinderAndWizard(t *testing.T) {
	cfg := config.DefaultConfig()

	snippets := []domain.Snippet{
		{
			ID:           "test.md#curl-endpoint",
			Name:         "Curl endpoint",
			Description:  "Query rest url",
			Body:         "curl -X <@method:?GET> <@url>",
			Language:     "sh",
			SourceFile:   "test.md",
			RelativePath: "test.md",
			Placeholders: []domain.Placeholder{
				{Name: "method", IsDefault: true, DefaultValue: "GET"},
				{Name: "url"},
			},
		},
	}

	files := []domain.SnippetFile{
		{
			Path:         "test.md",
			RelativePath: "test.md",
			Snippets:     snippets,
		},
	}

	m := NewAppModel(cfg, snippets, files)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Wait for the UI to render and verify fuzzy finder mode and preview pane
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Curl endpoint")) && bytes.Contains(bts, []byte("Code:"))
	}, teatest.WithDuration(1*time.Second))

	// Press Enter to select the only snippet, which enters Wizard Mode
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait and verify we are in Wizard Mode prompting for 'method'
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Variable: method"))
	}, teatest.WithDuration(1*time.Second))

	// Press Enter to accept the default value "GET" for 'method'
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait and verify we are prompting for 'url'
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Variable: url"))
	}, teatest.WithDuration(1*time.Second))

	// Type the URL character by character
	for _, char := range "https://google.com" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
	}

	// Press Enter to submit URL
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for the program to terminate on completion
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	// Verify the final model state
	finalModel := tm.FinalModel(t).(*AppModel)
	if !finalModel.Done {
		t.Error("expected model to be marked as done")
	}

	expectedSnippet := "curl -X GET https://google.com"
	if finalModel.FinalSnippet != expectedSnippet {
		t.Errorf("expected rendered snippet %q, got %q", expectedSnippet, finalModel.FinalSnippet)
	}

	// Verify final output indicates completion
	finalOut, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatalf("failed to read final output: %v", err)
	}
	if !bytes.Contains(finalOut, []byte("Snippet copied to clipboard!")) {
		t.Errorf("expected completion message in final output, got: %s", string(finalOut))
	}
}


func TestFinderPaneLayout(t *testing.T) {
    cfg := config.DefaultConfig()
    snippets := []domain.Snippet{{
        ID:          "test.md#demo",
        Name:        "Demo",
        Description: "Demo snippet",
        Body:        "echo hello",
        Language:    "sh",
        RelativePath:"test.md",
    }}
    files := []domain.SnippetFile{{
        Path:         "test.md",
        RelativePath: "test.md",
        Snippets:     snippets,
    }}
    m := NewAppModel(cfg, snippets, files)
    // Ensure finder mode is active
    out := m.View()
    // Verify left list contains the snippet name and right preview contains code block header
    if !strings.Contains(out, "Demo") {
        t.Fatalf("expected left pane to contain snippet name")
    }
    if !strings.Contains(out, "Code:") {
        t.Fatalf("expected right pane to contain code header")
    }
    if !strings.Contains(out, "│") {
        t.Fatalf("expected vertical separator between panes")
    }
}
