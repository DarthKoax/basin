# Basin

An interactive terminal strand (snippets) manager and template fill-in tool written in Go. Browse, search, and fill in markdown-based command strands from a sleek TUI.

## Features

- **Plain Markdown Storage** — strands stored as human-readable `.md` files
- **Interactive TUI** — built with [Bubble Tea](https://github.com/charmbracelet/bubbletea); fuzzy search, folder-tree browser, and tag navigation
- **Mouse Navigation** — click to select, scroll to navigate, drag to resize panels
- **Interactive Variable Resolution (Wizard)** — sequentially prompts for placeholder values with support for built-in suggestions, static lists, and command-backed suggestions
- **System Clipboard Integration** — automatically copies the rendered strand to the clipboard
- **External Editor Support** — open the source file in your editor with `Ctrl+E` and return with your selection retained
- **Code Block Styling** — fenced code blocks render with background colour and monospace font

## Installation

```bash
go install ./cmd/basin
```

## Quick Start

```bash
# Scaffold starter strand directories and files
basin init

# Launch the interactive TUI
basin
# or
basin execute
```

## Commands

| Command | Description |
|---------|-------------|
| `basin` / `basin execute` | Launch the interactive TUI |
| `basin init` | Scaffold starter strand directories and config |
| `basin lint` | Check strand files for syntax and variable errors |

### Flags

| Flag | Description |
|------|-------------|
| `--no-stdout` | Disable printing the strand to stdout on completion |
| `--no-clipboard` | Disable copying the strand to clipboard on completion |
| `--no-auto-copy` | Disable auto-copying strand body when highlighted |

## Strand File Format

Strands are plain markdown `.md` files. Each strand is declared by a level-two heading (`##`). The executable body is the first fenced code block under the heading that does **not** use the `text` language tag.

```markdown
---
name: My Strands
tags: [docker, devops]
variables:
  container:
    default: "my-app"
    suggestions: ["$(docker ps --format '{{.Names}}')"]
---

## Restart a container

Restart a running Docker container by name.

```sh
docker restart <$container>
```
```

### YAML Frontmatter

Frontmatter is optional. Supported fields:

| Field | Description |
|-------|-------------|
| `name` | Display name for the file |
| `tags` | List of tags for filtering |
| `variables` | Per-placeholder configuration (default, suggestions, command) |

### Placeholder Syntax

| Syntax | Description |
|--------|-------------|
| `<$name>` | Environment variable reference (falls back to `$name`) |

### Variable Resolution Priority

1. **Inline** — frontmatter `variables` block (per-file)
2. **Config** — `~/.config/basin/config.toml` `[variables]` section
3. **Built-ins** — file and directory names from the current working directory

## Configuration

Configuration lives at `~/.config/basin/config.toml`:

```toml
[paths]
snippets = ["/path/to/my/strands"]

[variables]
# Global variable defaults

[suggestion_commands]
timeout_ms = 2000
allow_commands = true

[output]
stdout = true
clipboard = true

[behavior]
auto_copy_on_select = true
```

Strand roots are discovered from (in order):
1. `config.toml` `[paths].snippets`
2. `BASIN_PATH` environment variable (`:` separated, `;` on Windows)
3. `~/.config/basin/strands`

## TUI Modes

| Mode | Purpose | Key Bindings |
|------|---------|--------------|
| **Finder** | Fuzzy search across all strands | `Enter`: wizard, `Tab`: focus preview, `Ctrl+T`: switch mode |
| **Browser** | Directory tree navigation | `Enter`: open dir/wizard, `Esc`: go up, `Ctrl+T`: switch mode |
| **Tags** | Browse strands by tag | `Enter`: select tag/strand, `Esc`: back to tags list |
| **Wizard** | Interactive variable resolution | `Enter/Tab`: next var, `Shift+Tab`: prev var, `Esc`: cancel |
| **Command Palette** | Quick command access | `Ctrl+P`: open, `Enter`: execute, `Esc`: close |

## Development

```bash
go build ./...           # build all packages
go run ./cmd/basin       # run the application
go test ./...            # run all tests
go vet ./...             # check for bugs
golangci-lint run        # lint (if installed)
```

## License

MIT
bob
