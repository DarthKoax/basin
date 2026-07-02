# Peanutclone: Development Plan

This document outlines the design, architecture, implementation roadmap, and testing strategy for `peanutclone`, a Go-based clone of the `peanutbutter` terminal snippet manager.

---

## 1. Core Architecture & Modules

The application will be structured as modular packages in Go to isolate responsibilities and make testing straightforward.

```mermaid
graph TD
    CLI[cmd/pb CLI] --> TUI[internal/tui Bubble Tea App]
    CLI --> Parser[internal/parser MD/YAML Parser]
    TUI --> Resolver[internal/resolver Var Resolver]
    TUI --> Search[internal/search Fuzzy Engine]
    Resolver --> CmdExec[os/exec Shell Command]
    TUI --> Clipboard[internal/clipboard Writer]
```

### Module Descriptions:
1. **`internal/domain`**: Holds the fundamental data types.
   - `Snippet`: Name, description, raw body block, file location, and list of placeholders.
   - `SnippetFile`: File path, metadata tags, and slice of parsed snippets.
   - `VariableSpec`: Configuration rules (default, suggestions list, suggestion command).
2. **`internal/parser`**: Parses Markdown files and handles frontmatter.
   - Scans markdown for `##` headers to split snippets.
   - Reads YAML frontmatter (metadata name, tags, variables config).
   - Extracts placeholders (`<@name>`, `<@name:?default>`, `<@name:cmd>`) from the first non-`text` code fence.
3. **`internal/config`**: Reads and manages local configurations.
   - Loads `/home/koax/.config/peanutbutter/config.toml` (overridable with environment variables).
   - Resolves global variables, timeouts, custom paths, and formatting styles.
4. **`internal/resolver`**: Resolves placeholder values and evaluates dynamic inputs.
   - Merges inline specs, file-local frontmatter, and global config.
   - Executes suggestion commands in `bash -c` with context-bounded timeouts.
   - Supports dependent variables (`<#upstream>` / `<#upstream:raw>`) by updating and re-evaluating downstream variable commands dynamically as user input is confirmed.
5. **`internal/search`**: Implements search and navigation.
   - Fuzzy finding over titles, content, tags, and bodies.
   - Structured file tree navigation.
   - Tag listing dashboard.
6. **`internal/tui`**: The Interactive Interface (using Charm Bubble Tea).
   - State machine containing:
     - **Finder View**: Input box and listing for fuzzy searching.
     - **Browser View**: Hierarchical folder directory listing.
     - **Tags View**: Filtering snippets by metadata tag.
     - **Wizard View**: Sequence of pages to prompt for missing variables, showing a selection list for suggestions (with command evaluation status) and standard input box.
7. **`internal/clipboard`**: Writes text to the clipboard cross-platform.
   - Uses `github.com/atotto/clipboard` in Go.

---

## 2. Key Libraries & Dependencies

| Purpose | Library | Reason / Role |
| :--- | :--- | :--- |
| **CLI Argument Parsing** | `github.com/spf13/cobra` | High-quality standard CLI utility for commands and completions. |
| **TUI Runtime Engine** | `github.com/charmbracelet/bubbletea` | Best-in-class Elm-architecture framework for Go terminal apps. |
| **TUI Inputs & Lists** | `github.com/charmbracelet/bubbles` | Standard components (`textinput`, `list`, `viewport`, `key`) to build views. |
| **TUI Layout & Styles** | `github.com/charmbracelet/lipgloss` | Declarative terminal layout and text styling library. |
| **Fuzzy Matching** | `github.com/sahilm/fuzzy` | Quick, efficient fuzzy match engine for Go slices. |
| **YAML Frontmatter** | `gopkg.in/yaml.v3` | Parsing YAML frontmatter blocks inside Markdown files. |
| **TOML Config File** | `github.com/pelletier/go-toml/v2` | Fast, compliant parser for `config.toml` configuration files. |
| **Clipboard Copy** | `github.com/atotto/clipboard` | Direct clipboard access across Windows, macOS, and Linux. |
| **TUI Integration Test**| `github.com/charmbracelet/x/exp/teatest` | Simulation harness for verifying Bubble Tea models, keys, and views. |

---

## 3. Step-by-Step Implementation Phases

```
┌───────────────────────────────────────────────────────────────┐
│ Phase 1: Setup & Boilerplate (cli flags, config reader)       │
├───────────────────────────────────────────────────────────────┤
│ Phase 2: MD & YAML Parser (snippet and frontmatter parsing)    │
├───────────────────────────────────────────────────────────────┤
│ Phase 3: Variable Resolution Engine (suggestions, dependency) │
├───────────────────────────────────────────────────────────────┤
│ Phase 4: Core TUI Navigation (fuzzy find list, file tree, tags)│
├───────────────────────────────────────────────────────────────┤
│ Phase 5: TUI Wizard Component (sequential variable prompts)  │
├───────────────────────────────────────────────────────────────┤
│ Phase 6: Clipboard Integration & Commands                    │
├───────────────────────────────────────────────────────────────┤
│ Phase 7: Complete Testing (unit tests and teatest integration)│
└───────────────────────────────────────────────────────────────┘
```

### Phase 1: Setup & Skeleton CLI
- Initialize `go mod init peanutclone` in the workspace directory.
- Create basic directory layout matching the repository layout.
- Build the `pb` CLI command skeleton (`cmd/pb/main.go` using Cobra).
- Implement config file parser (`internal/config`) to parse TOML configurations.

### Phase 2: Markdown & Frontmatter Parser
- Write parser logic that reads a Markdown snippet file.
- Scan for `##` sections. Identify description block and code fences.
- Extract YAML frontmatter at the top of the file using `gopkg.in/yaml.v3`.
- Parse placeholders inside the code block matching regex `<@([a-zA-Z0-9_-]+)(:(\?.+|[^>]+))?>` to extract variables and their inline sources.
- Exclude `text` fenced code blocks from executable snippets, using them only as preview content.
- Verify with unit tests.

### Phase 3: Variable & Template Resolver
- Establish priority resolution (Inline -> Frontmatter -> Config -> Built-in suggestions).
- Implement built-in directory/file suggestions using `os.ReadDir` in the current working directory.
- Build runner to execute suggestion commands using `exec.CommandContext` with a configurable timeout.
- Implement dependent variables processor:
  - Parse `<#name>` (shell-escaped) and `<#name:raw>` (literal splice) inside command suggestions.
  - Assert that references point to variables ordered prior to the current variable (prevent cyclic/forward dependencies).
- Verify template interpolation with tests.

### Phase 4: TUI Selection Modes
- Build main Bubble Tea application model.
- **Fuzzy Finder**: Displays all parsed snippets. Pressing `ctrl+t` toggles active view.
- **Browser**: Shows snippets in a hierarchical list corresponding to folders and files.
- **Tags View**: Groups snippets by their tags and counts them. Allows selection of a tag to filter.
- Support Helix-style operator searches in fuzzy finder (`tag:docker`, `path:...`, `name:...`, `snippet:`).

### Phase 5: TUI Variable Fill-in Wizard
- If the selected snippet requires variables, transition the TUI to the **Wizard view**.
- The Wizard maintains an input stack of placeholders.
- For each placeholder:
  - Render placeholder name and description.
  - Display suggestion list (static list, files/dirs list, or stdout from executed commands).
  - Support list filtering and selection using keyboard arrow keys.
  - Allow typing custom text inside a `textinput` component.
  - Support multi-line input insertion via **Alt+Enter** (or paste).
  - Pressing `Enter` or `Tab` locks the value, re-evaluates dependent variables downstream, and advances.
  - Pressing `Shift+Tab` navigates backward to correct previous inputs.
- When all variables are filled, compile the final rendered template and transition to Clipboard Copy.

### Phase 6: Clipboard Integration & Commands
- Copy final content to system clipboard using `github.com/atotto/clipboard`.
- Output selected snippet to stdout for capturing by shell buffers (similar to the `--bash` eval capture).
- Support terminal raw-mode restore on exits.

### Phase 7: Verification & Testing
- Write comprehensive unit tests for parser, resolver, config, and domain modules.
- Create TUI integration tests using `teatest` (details below).

---

## 4. Testing & Integration Strategy

### Unit Testing Guidelines
- Use the standard `testing` package with `testify/assert` or simple comparison checks.
- Keep tests completely isolated; avoid modifying actual user files or reading production config directories. Mock all filesystem and command invocations.

### TUI Integration Testing (Visible & Navigable)
To satisfy the requirement that "the CUI must be integrated tested to ensure tui elements are visible and it is navigable," we will write tests using `github.com/charmbracelet/x/exp/teatest` or program inputs.

#### teatest Example Harness:
```go
package tui_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"peanutclone/internal/domain"
	"peanutclone/internal/tui"
)

func TestTUI_FuzzyFinderNavigation(t *testing.T) {
	// Initialize sample snippets
	snippets := []domain.Snippet{
		{Name: "List directory contents", Body: "ls -la"},
		{Name: "Check disk usage", Body: "df -h"},
	}

	// Create bubbletea model
	m := tui.NewAppModel(snippets)

	// Run teatest simulator
	tm := teatest.NewTestModel(t, m, teatest.WithWidth(80), teatest.WithHeight(24))

	// Send key events to navigate down the list
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	
	// Wait and read terminal buffer
	time.Sleep(100 * time.Millisecond)
	out, err := tm.Output()
	if err != nil {
		t.Fatalf("failed to read output buffer: %v", err)
	}

	// Verify that the second item is selected/visible in the UI output
	if !strings.Contains(string(out), "Check disk usage") {
		t.Errorf("expected 'Check disk usage' to be visible, got: %s", string(out))
	}

	// Close the test model
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.WaitFinished(t, 2*time.Second)
}

func TestTUI_WizardFillIn(t *testing.T) {
	// Snippet requiring template variables
	snippets := []domain.Snippet{
		{
			Name: "Curl endpoint",
			Body: "curl -X <@method> <@url>",
			Placeholders: []domain.Placeholder{
				{Name: "method", DefaultValue: "GET"},
				{Name: "url"},
			},
		},
	}

	m := tui.NewAppModel(snippets)
	tm := teatest.NewTestModel(t, m, teatest.WithWidth(80), teatest.WithHeight(24))

	// Ensure wizard prompts for method first
	out, _ := tm.Output()
	if !strings.Contains(string(out), "Enter value for: method") {
		t.Fatalf("expected wizard to start with 'method' variable prompt, got: %s", string(out))
	}

	// Press Enter to accept default "GET"
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Ensure we move to 'url'
	time.Sleep(50 * time.Millisecond)
	out, _ = tm.Output()
	if !strings.Contains(string(out), "Enter value for: url") {
		t.Fatalf("expected wizard to progress to 'url' variable prompt, got: %s", string(out))
	}

	// Type URL
	for _, char := range "https://api.github.com" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
	}

	// Submit URL
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify TUI completes and exits cleanly
	tm.WaitFinished(t, 2*time.Second)
}
```

This integration test design allows us to run standard Go test scripts that boot up headless versions of the TUI, send keystrokes, and inspect the screen contents to verify visibility, search filtering, and list navigation.
