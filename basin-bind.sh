#!/usr/bin/env bash

# Basin keybinding setup for bash
# Source this file in your .bashrc: source /path/to/basin-bind.sh

__basin_keybind() {
    local basin_path
    basin_path="$(dirname "${BASH_SOURCE[0]}")/basin"
    
    if [[ ! -x "$basin_path" ]]; then
        echo "Error: basin executable not found at $basin_path" >&2
        return 1
    fi
    
    "$basin_path" execute
}

# Bind Ctrl+B to trigger Basin
bind -x '"\C-b": __basin_keybind'
