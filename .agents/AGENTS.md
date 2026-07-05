# AGENTS.md

This file contains the context, requirements, development rules, and codebase map for **Basin**—a Go-based interactive terminal strand manager.

---

## 1. Project Philosophy & Goals

`Basin` is a command-line strand manager and template fill-in tool written in Go. It provides an interactive TUI for browsing, searching, and filling in markdown-based command strands.

### Core Objectives:
1. **Plain Markdown Storage**: Store all strand collections as standard, human-readable `.md` markdown files.
2. **CLI & TUI Interfaces**: Provide a sleek command-line interface and an inline TUI for interactive strand selection.
3. **Fuzzy Search & Navigation**: Offer instant fuzzy searching across titles, descriptions, tags, and bodies, alongside a structured folder-tree browser.
4. **Mouse Navigation**: The TUI is fully navigable with the mouse—click to select items, scroll to navigate lists and previews.
5. **Interactive Variable Resolution (Wizard)**: Interactively prompt the user for placeholder values (like `<@container>` or `<@host:rg . --files>`) sequentially in the TUI, supporting:
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

**No guessing. Surface tradeoffs. Maintain simplicity.**
- **Express Assumptions**: If a requirement is ambiguous (e.g., shell script behaviors, configuration layouts), state assumptions explicitly or ask.
- **Simplicity First**: Write the simplest code that solves the problem. Do not build speculative features, unnecessary generic wrappers, or unused abstractions.
- **Minimize Scope**: Do not expand scope unless asked. Keep components tightly bound to the features required.

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
peanutclone/
  ├── cmd/
  │   └── basin/
  │       └── main.go       # Entry point: handles CLI command routing & flags
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
  ├── AGENTS.md             # This rules and architecture map
  ├── PLAN.md               # Step-by-step project development plan
  ├── go.mod                # Module definitions
  └── go.sum                # Dependency verification hashes
```

---

## 5. Strand Parsing Rules

Refer to the original specification for markdown formats:
- Strand files are markdown `.md` files.
- Optional YAML frontmatter block at the top defined by `---` wrappers.
- A strand is declared by a level-two markdown header (`##`).
- Any description/preview text resides between the `##` header and the first code block.
- The executable strand body is the *first fenced code block* under that header that does not use the `text` language tag.
- Fenced code blocks with `text` are strictly reserved for preview-only examples and must not be used as executable bodies.

---

## 6. Testing Strategy

### Unit Testing
- Every module under `internal/parser`, `internal/resolver`, and `internal/config` must have `*_test.go` files with high coverage.
- Write unit tests for:
  - Frontmatter parser and variable spec merging.
  - Regex placeholder extraction: `<@name>`, `<@name:?default>`, `<@name:command>`.
  - Dependent variable template rendering: `<#upstream>` and `<#upstream:raw>`.
  - Resolution order logic (Inline -> Frontmatter -> Config -> Built-ins).

### TUI Integration Testing
- The TUI must be integration tested to ensure elements are visible, active, and navigable.
- **Testing framework**: Use `github.com/charmbracelet/x/exp/teatest` or mock terminal I/O buffers.
- Write tests that:
  - Simulate key inputs (`ctrl+t` to switch modes, arrow keys to navigate lists).
  - Verify that UI elements (lists, inputs, previews) render correctly to the output buffer.
  - Run the Wizard view and verify that entering values and pressing `enter` correctly moves to the next variable, and finally exits with the expected rendered clipboard output.
