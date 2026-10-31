package tui

import (
	"fmt"
	"strings"
	"testing"

	"basin/internal/config"
	"basin/internal/domain"
)

// TestPaneHeightWithScrollableContent tests that both panes maintain full height
// when the left pane has scrollable content
func TestPaneHeightWithScrollableContent(t *testing.T) {
	cfg := config.DefaultConfig()

	// Create many snippets to force scrolling in left pane
	snippets := make([]domain.Snippet, 100)
	for i := 0; i < 100; i++ {
		snippets[i] = domain.Snippet{
			Name:         fmt.Sprintf("snippet-%d", i),
			RelativePath: "test.md",
			Body:         "echo test",
			Language:     "sh",
		}
	}

	// Test various terminal heights to find where the issue occurs
	testHeights := []int{10, 12, 15, 18, 20, 25, 30}

	for _, termHeight := range testHeights {
		t.Run(fmt.Sprintf("Height_%d", termHeight), func(t *testing.T) {
			m := NewAppModel(cfg, nil, snippets, nil)
			m.termWidth = 120
			m.termHeight = termHeight
			m.mode = FinderMode
			m.finderFocusedPanel = 0

			view := stripANSI(m.View())
			lines := strings.Split(view, "\n")

			// Count the actual height of each pane by counting lines with │
			leftPaneHeight := 0
			rightPaneHeight := 0

			for _, line := range lines {
				if strings.Contains(line, "│") {
					parts := strings.Split(line, "│")
					if len(parts) >= 3 {
						leftPaneHeight++
					}
					if len(parts) >= 5 {
						rightPaneHeight++
					}
				}
			}

			// Both panes should have the same height
			if leftPaneHeight != rightPaneHeight {
				t.Errorf("Pane heights don't match! Left: %d, Right: %d", leftPaneHeight, rightPaneHeight)
			}

			// The pane height should match the expected available height
			_, _, expectedHeight := m.panelDims()
			if leftPaneHeight != expectedHeight {
				t.Errorf("Left pane height %d doesn't match expected %d (terminal height: %d)", 
					leftPaneHeight, expectedHeight, termHeight)
			}
			if rightPaneHeight != expectedHeight {
				t.Errorf("Right pane height %d doesn't match expected %d (terminal height: %d)", 
					rightPaneHeight, expectedHeight, termHeight)
			}

			t.Logf("Terminal: %d, Panes: %d, Expected: %d", termHeight, leftPaneHeight, expectedHeight)
		})
	}
}
