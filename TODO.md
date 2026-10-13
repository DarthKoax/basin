# Quality of Life Improvements TODO

## High Impact

### Smart Defaults & History ✅ COMPLETED
- [x] Remember last-used variable values per strand in `~/.config/basin/history.json`
- [x] Track recently used strands and surface them at the top of Finder
- [x] Add "frequently used" section or sort by usage frequency

### Enhanced Search
- [ ] Fuzzy search should match across description, tags, and body content (not just name)
- [ ] Support filter operators: `tag:docker`, `path:devops/`, `name:restart`
- [ ] Show search highlights in preview panel

### Variable Wizard Improvements
- [x] Pre-fill variables with last-used values from history
- [ ] Multi-line input support for variables (detect when value contains newlines)
- [ ] Tab completion for file/directory suggestions
- [ ] Validation hints (show expected format: "enter port number", "enter container name")

### Strand Management
- [ ] Quick strand creation: `basin new` opens editor with template
- [ ] Duplicate strand detection in lint mode
- [ ] Strand execution history with timestamps (last 20 runs)
- [ ] Copy strand source to clipboard (not just rendered output) with `Ctrl+Shift+C`

## Medium Impact

### Visual Polish
- [ ] Show variable count badge next to strand name in lists (e.g., "Restart Container [3 vars]")
- [ ] Animated transitions between modes (subtle fade/slide)
- [ ] Color-code strands by tag or directory
- [ ] Preview pane: syntax highlighting for more languages (auto-detect from code block)

### Navigation & Workflow
- [x] Keyboard shortcuts cheat sheet: `?` or `Ctrl+/` opens overlay
- [ ] Jump to strand by ID: `:123` or `g123` in Finder
- [ ] Bookmark/favorite strands with `Ctrl+F`
- [ ] Open strand in external viewer (not editor) with `Ctrl+V` - read-only mode
- [ ] Cycle through recent strands with `Alt+Left/Right`

### Variable Enhancements
- [ ] Conditional variables: `<$deploy_env>` prompts different follow-ups for "staging" vs "production"
- [ ] Variable validation rules (regex patterns, min/max values)
- [ ] Dependent variable chains: if `env=prod`, auto-set `cluster=prod-us-east-1`
- [ ] Show variable resolution order in wizard (Inline → Config → Built-in)

### Export & Integration
- [ ] Export strand as shell script: `basin export <strand-name> > script.sh`
- [ ] Copy as different formats: JSON, YAML, URL-encoded
- [ ] Integrate with shell history (add rendered strand to `~/.bash_history` or `~/.zsh_history`)
- [ ] Webhook support: trigger external service after strand execution

## Low Impact (Nice to Have)

### Configuration & Themes
- [ ] Theme presets: dark, light, solarized, gruvbox
- [ ] Customizable keybindings via config
- [ ] Font size adjustment in TUI (if terminal supports it)
- [ ] Panel width saved per-mode (Finder can be wider than Browser)

### Extensibility
- [ ] Plugin system for custom variable types (e.g., AWS profiles, Kubernetes contexts)
- [ ] Custom suggestion backends (HTTP API, database queries)
- [ ] Pre/post execution hooks (run commands before/after strand completion)

### Documentation & Onboarding
- [ ] Interactive tutorial: `basin tutorial` walks through features
- [ ] In-app help: `Ctrl+H` shows context-sensitive help for current mode
- [ ] Example strand library: `basin examples` downloads curated strand collection

### Performance & Reliability
- [ ] Strand file watching (auto-reload when files change)
- [ ] Graceful degradation when clipboard tools unavailable (show warning, don't block)
- [ ] Offline mode indicator (when network-dependent suggestions fail)

## Quick Wins (Start Here)

1. [x] Smart defaults from history - pre-fill wizard variables with last-used values
2. [x] Keyboard shortcuts overlay - 2 hours, high visibility
3. [x] Recent strands tracking - surface frequently used strands at top of Finder
4. [ ] Multi-line variable input - 3 hours, solves real pain point
5. [ ] Search filter operators - 6 hours, power-user feature
