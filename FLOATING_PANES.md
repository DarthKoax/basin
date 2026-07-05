# Floating Panes in Basin

## Overview

Floating panes are centered overlay windows that appear on top of the existing TUI without disrupting the underlying layout. They can be used for alerts, menus, help text, or any other task that requires temporary focused attention.

## Features

- **Centered Positioning**: Automatically centered in the terminal window
- **Non-Destructive**: Does not break or modify the panes underneath
- **Selectable**: Can receive focus and handle keyboard input
- **Dismissible**: Can be closed with Esc, Enter, or Q keys
- **Customizable**: Configurable title, content, width, and height

## Usage

### Keyboard Shortcut

Press `Ctrl+F` in any mode (Finder, Browser, Tags) to show the demo floating pane.

### Command Palette

1. Press `Ctrl+P` to open the command palette
2. Type "float" or navigate to "Show Floating Pane"
3. Press Enter to display the floating pane

### Programmatic API

```go
// Show a floating pane
m.showFloatingPane("Title", []string{
    "Line 1",
    "Line 2",
    "Line 3",
}, 50, 12) // width, height

// Hide the floating pane
m.hideFloatingPane()
```

## Implementation Details

### State Management

The floating pane state is stored in the `AppModel`:

```go
type floatingPane struct {
    active  bool
    title   string
    content []string
    width   int
    height  int
}
```

### Rendering

The floating pane is rendered as an overlay on top of the existing view using a line-by-line approach:

1. The base view is rendered normally (all panes, headers, footers)
2. If a floating pane is active, it's rendered on top by:
   - Calculating the centered position based on terminal dimensions
   - For lines covered by the floating pane: writing spaces for the left offset, then the floating pane content
   - For lines not covered: writing the base view content normally
3. Each line is properly terminated with a newline
4. Content is styled with a golden rounded border and dark background

This approach ensures that:
- The floating pane completely covers the underlying content
- ANSI codes are properly contained within each line
- The underlying panes are not broken or modified
- The floating pane is properly centered

### Key Handling

When a floating pane is active:
- All other keybindings are disabled
- Only Esc, Enter, and Q are handled (to dismiss the pane)
- This prevents accidental interactions with the underlying UI

## Use Cases

### Alerts and Notifications

```go
m.showFloatingPane("Warning", []string{
    "This action cannot be undone.",
    "Press Enter to continue or Esc to cancel.",
}, 60, 8)
```

### Help Text

```go
m.showFloatingPane("Keyboard Shortcuts", []string{
    "Ctrl+T: Switch modes",
    "Ctrl+P: Command palette",
    "Ctrl+E: Edit source file",
    "Ctrl+F: Show floating pane",
    "Ctrl+D: Quit",
}, 50, 15)
```

### Confirmation Dialogs

```go
m.showFloatingPane("Confirm Delete", []string{
    "Are you sure you want to delete this strand?",
    "",
    "Enter: Yes  Esc: No",
}, 50, 10)
```

### Menus

```go
m.showFloatingPane("Actions", []string{
    "1. Edit strand",
    "2. Copy to clipboard",
    "3. Delete strand",
    "4. Cancel",
}, 40, 12)
```

## Testing

Three test cases verify the floating pane functionality:

1. **TestFloatingPane_ShowAndHide**: Verifies show/hide state management
2. **TestFloatingPane_DismissWithEsc**: Verifies keyboard dismissal
3. **TestFloatingPane_CenteredPosition**: Verifies centering logic

Run tests with:
```bash
go test ./internal/tui -v -run TestFloatingPane
```

## Future Enhancements

Potential improvements for floating panes:

- **Mouse Support**: Click to dismiss or interact with buttons
- **Multiple Panes**: Stack multiple floating panes
- **Input Fields**: Add text input capabilities
- **Buttons**: Interactive buttons with actions
- **Scrolling**: Scrollable content for long messages
- **Animations**: Fade in/out effects
- **Custom Styles**: Configurable colors and borders

## Technical Notes

- The floating pane uses lipgloss for styling (rounded border, golden color)
- Content is padded and properly sized to fit within the terminal
- The overlay is character-based, ensuring compatibility with all terminals
- The pane automatically adjusts if the terminal is resized
