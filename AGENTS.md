# AGENTS.md

This file contains the context, requirements, development rules, and codebase map for **Basin**—a Go-based interactive terminal strand (snippets) manager.

---

## 1. Project Philosophy & Goals

`Basin` is a command-line strand manager and template fill-in tool written in Go. It provides an interactive TUI for browsing, searching, and filling in markdown-based command strands.

### Core Objectives:
1. **Plain Markdown Storage**: Store all strand collections as standard, human-readable `.md` markdown files.
2. **CLI & TUI Interfaces**: Provide a sleek command-line interface and an inline TUI for interactive strand selection.
3. **Fuzzy Search & Navigation**: Offer instant fuzzy searching across titles, descriptions, tags, and bodies, alongside a structured folder-tree browser.
4. **Mouse Navigation**: The TUI is fully navigable with the mouse—click to select items, scroll to navigate lists and previews.
5. **Interactive Variable Resolution (Wizard)**: Interactively prompt the user for placeholder values (like `<$container>` or `<$host>`) sequentially in the TUI, supporting:
   - Built-in suggestions (files, directories).
   - Static suggestion lists (from config/frontmatter).
   - Command-backed suggestions (evaluated via shell).
   - Dependent variables (where downstream suggestions rely on upstream inputs, e.g., `<#secret>`).
6. **System Clipboard Integration**: Copy the fully rendered strand to the system clipboard upon completion.
7. **External Editor Support**: Open the source markdown file in the system's default editor (via `Ctrl+E`), returning to the app with previous selection retained.
8. **Code Block Styling**: Fenced code blocks (```) render with a subtle background colour and monospace font. Inline code (backtick `) renders with a background highlight.
9. **Rigorous Quality**: Maintain unit-test coverage for core parsing and rendering, and integration tests to verify TUI visibility and navigability.

---

## 2. Think Before Coding

- **Prioritize Simplicity**: Implement the simplest solution that solves the problem. Avoid speculative features, unnecessary abstractions, or generic wrappers. The TUI state machine and resolver chain should stay flat and readable.
- **Minimize Scope**: Keep components tightly bound to required features. Do not expand scope unless explicitly requested — e.g., don't add new TUI modes, placeholder syntax variants, or config options without being asked.


---

## 3. Go-Specific Development Rules

Follow these rules when writing Go code:

### Formatting & Tooling
- Use standard `go fmt` for all files.
- Run `go vet` and `golangci-lint` to check for bugs and style issues.
- Keep dependency counts minimal. Rely on the standard library where appropriate.

### Code Style
- Document public APIs (functions, interfaces, structs, packages) with standard Go doc comments.
- Errors must be handled explicitly and wrapped with context: `fmt.Errorf("parsing frontmatter: %w", err)`.
- Restrict package scopes appropriately; keep internal implementation details in `internal/` packages and expose clean APIs.

### Concurrency
- Suggestion commands and background lookups should run safely. Use context timeouts to prevent hanging processes:
  - Default timeout for running suggestion commands must be 2000ms.
  - Implement command cancellation using `context.WithTimeout`.

---

## 4. Repository Structure

The project follows a standard Go directory layout:

```text
basin/
  ├── cmd/
  │   └── basin/
  │       └── main.go       # Entry point: cobra CLI routing (execute, init, lint)
  ├── internal/
  │   ├── config/           # TOML parser/loader for ~/.config/basin/config.toml
  │   ├── domain/           # Core models (Snippet, SnippetFile, Variable, Placeholder)
  │   ├── parser/           # Markdown and YAML frontmatter parsing logic
  │   ├── resolver/         # Engine for computing default values and running suggestion commands
  │   ├── clipboard/        # Cross-platform clipboard writer wrapper
  │   ├── search/           # Fuzzy matcher and folder tree navigation logic
  │   └── tui/              # TUI views and wizard (Bubble Tea + Bubbles + Lip Gloss)
  │       ├── app.go        # Main Bubble Tea app state machine
  │       └── app_test.go   # TUI integration tests
  ├── examples/             # Example strand files
  ├── AGENTS.md             # This rules and architecture map
  ├── go.mod                # Module definitions
  └── go.sum                # Dependency verification hashes
```

### Data Flow

```text
CLI args → cobra (cmd/basin/main.go) → config.LoadConfig()
  ↓
parser.DiscoverAndParse(roots) → []domain.SnippetFile
  ↓
tui.NewAppModel(cfg, roots, snippets, files) → Bubble Tea program
  ↓
TUI state machine (5 modes) → user selection → resolver.RenderSnippet()
  ↓
clipboard.WriteAll() + stdout output
```

---

## 5. Strand Parsing Rules

- Strand files are markdown `.md` files.
- YAML frontmatter at the top of a markdown file is optional.
- A strand is declared by a level-two markdown header (`##`).
- Any description/preview text resides between the `##` header and the first code block.
- The executable strand body is the *first fenced code block* under that header that does not use the `text` language tag.
- Fenced code blocks with `text` are strictly reserved for preview-only examples and must not be used as executable bodies.

### Placeholder Syntax

Placeholders use the `< >` boundary characters (configurable via frontmatter):

- `<$name>` — environment variable reference (falls back to `$name` if not provided)
- `<#upstream>` — dependent variable reference (shell-quoted on substitution)
- `<#upstream:raw>` — dependent variable reference (no quoting)

Frontmatter `variables` block can define per-placeholder behaviour:

```yaml
variables:
  container:
    default: "my-app"
    suggestions: ["$(docker ps --format '{{.Names}}')"]
  host:
    command: "rg . --files"
```

### Resolution Priority

1. **Inline** — frontmatter `variables` block (per-file)
2. **Config** — `~/.config/basin/config.toml` `[variables]` section
3. **Built-ins** — `file` and `directory` names from the current working directory

---

## 6. Testing Strategy

### Unit Testing
- Every module under `internal/parser`, `internal/resolver`, `internal/config`, and `internal/search` must have `*_test.go` files with high coverage.
- Write unit tests for:
  - Frontmatter parser and variable spec merging.
  - Regex placeholder extraction: `<$name>`, `<#upstream>`, `<#upstream:raw>`.
  - Dependent variable template rendering and shell quoting.
  - Resolution order logic (Inline -> Config -> Built-ins).
  - Search token parsing and fuzzy matching across name, description, tags, body, and path.

### TUI Integration Testing
- The TUI must be integration tested to ensure elements are visible, active, and navigable.
- **Testing framework**: Use `github.com/charmbracelet/x/exp/teatest` or mock terminal I/O buffers.
- Write tests that:
  - Simulate key inputs (`ctrl+t` to switch modes, arrow keys to navigate lists).
  - Verify that UI elements (lists, inputs, previews) render correctly to the output buffer.
  - Run the Wizard view and verify that entering values and pressing `enter` correctly moves to the next variable, and finally exits with the expected rendered clipboard output.

---

## 7. TUI Architecture

The TUI is a Bubble Tea application with 5 modes:

| Mode | Purpose | Keybindings |
|------|---------|-------------|
| **Finder** | Fuzzy search across all strands | `Enter`: wizard, `Tab`: focus preview, `Ctrl+T`: switch mode |
| **Browser** | Directory tree navigation | `Enter`: open dir/wizard, `Esc`: go up, `Ctrl+T`: switch mode |
| **Tags** | Browse strands by tag | `Enter`: select tag/strand, `Esc`: back to tags list |
| **Wizard** | Interactive variable resolution | `Enter/Tab`: next var, `Shift+Tab`: prev var, `Esc`: cancel |
| **Command Palette** | Quick command access | `Ctrl+P`: open, `Enter`: execute, `Esc`: close |

### Key Invariants

- **Draggable divider**: Users can drag the vertical divider between panels to resize. Width is stored in `runtimeWidthPct` (5-90%).
- **Auto-copy on select**: When enabled, strand body is copied to clipboard as the user navigates.
- **Editor reload**: After editing a strand file externally (`Ctrl+E`), strands are re-parsed and selection is retained.
- **Overlay rendering**: The Wizard renders as a centered overlay dialog, not a separate mode view.
- **Mouse support**: Full mouse navigation — click to select, scroll to navigate, drag to resize panels, drag-select in preview to copy text.

---

## 8. Commands

```bash
go build ./...          # build all packages
go run ./cmd/basin      # run the application
go test ./...           # run all tests
go test ./internal/...  # run internal package tests
go vet ./...            # check for bugs
golangci-lint run       # lint (if installed)
```
