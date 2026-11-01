package tui

import (
	"basin/internal/config"
	"basin/internal/domain"
	"strings"
	"testing"
)

func TestWrapLines(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		width    int
		expected []string
	}{
		{
			name:     "no wrapping needed",
			lines:    []string{"short line"},
			width:    20,
			expected: []string{"short line"},
		},
		{
			name:     "single line needs wrapping",
			lines:    []string{"this is a very long line that needs wrapping"},
			width:    20,
			expected: []string{"this is a very long", "line that needs", "wrapping"},
		},
		{
			name:     "multiple lines",
			lines:    []string{"short", "this is a very long line that needs wrapping", "another short"},
			width:    20,
			expected: []string{"short", "this is a very long", "line that needs", "wrapping", "another short"},
		},
		{
			name:     "exact width",
			lines:    []string{"exactly twenty ch"},
			width:    20,
			expected: []string{"exactly twenty ch"},
		},
		{
			name:     "empty lines",
			lines:    []string{"", "text", ""},
			width:    20,
			expected: []string{"", "text", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapLines(tt.lines, tt.width)
			if len(result) != len(tt.expected) {
				t.Errorf("wrapLines() returned %d lines, expected %d\nGot: %v\nExpected: %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("wrapLines() line %d = %q, expected %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestFullscreenLineWrapping(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewAppModel(cfg, nil, []domain.Snippet{}, []domain.SnippetFile{})
	m.termWidth = 80
	m.termHeight = 30
	m.fullscreenActive = true
	m.mode = FinderMode

	// Create a long line that should wrap
	longLine := strings.Repeat("word ", 20)
	snip := domain.Snippet{
		Name:     "test",
		Body:     longLine,
		Language: "text",
	}
	m.allSnippets = []domain.Snippet{snip}

	// Render the view
	view := m.View()

	// In fullscreen mode, the long line should be wrapped
	// Check that we have more than one line with content
	lines := strings.Split(view, "\n")
	contentLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.Contains(line, "┌") &&
			!strings.Contains(line, "└") && !strings.Contains(line, "│") &&
			!strings.Contains(line, "Enter:") {
			contentLines++
		}
	}

	// We should have multiple content lines due to wrapping
	if contentLines < 2 {
		t.Errorf("Expected long line to be wrapped into multiple lines, got %d content lines", contentLines)
	}
}
