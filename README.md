# Basin

Interactive terminal strand (snippet) manager written in Go. Browse, search, and fill in markdown-based command strands with an interactive TUI.

## Features

- Plain markdown storage for all strands
- Fuzzy search across titles, descriptions, tags, and bodies
- Folder tree browser with mouse navigation
- Interactive variable resolution wizard
- System clipboard integration
- External editor support (`Ctrl+E`)
- Code block syntax highlighting

## Installation

```bash
go install github.com/DarthKoax/basin/cmd/basin@latest
```

Or build from source:

```bash
git clone https://github.com/DarthKoax/basin.git
cd basin
go build ./cmd/basin
```

## Usage

```bash
basin execute          # Launch the interactive TUI
basin init             # Scaffold starter strand directories and files
basin lint             # Check strand files for syntax and variable errors
```

### TUI Keybindings

| Mode | Key | Action |
|------|-----|--------|
| Finder/Browser | `Ctrl+T` | Switch between Finder and Browser modes |
| Finder | `Enter` | Open variable wizard for selected strand |
| Finder | `Tab` | Focus preview panel |
| Browser | `Enter` | Open directory or launch wizard |
| Browser | `Esc` | Go up one directory |
| Wizard | `Enter` / `Tab` | Next variable |
| Wizard | `Shift+Tab` | Previous variable |
| Wizard | `Esc` | Cancel and return |
| Any | `Ctrl+P` | Open command palette |
| Any | `Ctrl+E` | Open strand file in external editor |

### Tmux Popup Integration

Add this to your `~/.tmux.conf` to open Basin in a tmux popup:

```tmux
bind-key ` display-popup -w 80% -h 80% -E 'bash -c "exec /home/koax/code/basin/main"'
```

Then press `<prefix> + `` to launch Basin in a floating popup.

### Keyboard Shortcut (Bash)

Source the included `basin-bind.sh` in your `~/.bashrc` to bind `Ctrl+B`:

```bash
source /path/to/basin-bind.sh
```

## Configuration

Configuration lives at `~/.config/basin/config.toml`:

```toml
[paths]
# strands = ["/path/to/my/extra/strands"]

[suggestion_commands]
timeout_ms = 2000
allow_commands = true
```

### Variables

Define variable defaults and suggestions in strand frontmatter or config:

```yaml
variables:
  container:
    default: "my-app"
    suggestions: ["$(docker ps --format '{{.Names}}')"]
  host:
    command: "rg . --files"
```

## Strand Files

Strands are markdown `.md` files. Each strand is a level-two heading (`##`) with an optional description and a fenced code block body:

````markdown
## List directory contents

Show detailed directory listing.

```sh
ls -lsha <$path>
```
````

Placeholders use `<$name>` syntax for environment variables and `<#upstream>` for dependent variables.

## Development

```bash
go build ./...          # build all packages
go test ./...           # run all tests
go vet ./...            # check for bugs
golangci-lint run       # lint
```

## License

MIT
