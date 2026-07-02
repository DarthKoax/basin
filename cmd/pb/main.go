package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"peanutclone/internal/config"
	"peanutclone/internal/domain"
	"peanutclone/internal/parser"
	"peanutclone/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "pb",
	Short: "peanutclone - a Go clone of the peanutbutter snippet manager",
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: execute finder TUI
		runExecute()
	},
}

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Run the interactive terminal snippet selector",
	Run: func(cmd *cobra.Command, args []string) {
		runExecute()
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold starter snippet directories and files",
	Run: func(cmd *cobra.Command, args []string) {
		runInit()
	},
}

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Check configured snippet files for syntax and variable errors",
	Run: func(cmd *cobra.Command, args []string) {
		runLint()
	},
}

func init() {
	rootCmd.AddCommand(executeCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(lintCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getSnippetRoots(cfg *config.Config) []string {
	var roots []string

	// 1. Config paths
	roots = append(roots, cfg.Paths.Snippets...)

	// 2. PEANUTBUTTER_PATH env var
	envPath := os.Getenv("PEANUTBUTTER_PATH")
	if envPath != "" {
		separator := ":"
		if filepath.Separator == '\\' {
			separator = ";"
		}
		parts := strings.Split(envPath, separator)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				roots = append(roots, part)
			}
		}
	}

	// 3. XDG default path (~/.config/peanutbutter/snippets/)
	home, err := os.UserHomeDir()
	if err == nil {
		xdgPath := filepath.Join(home, ".config", "peanutbutter", "snippets")
		roots = append(roots, xdgPath)
	}

	// Deduplicate and filter existing directories
	seen := make(map[string]bool)
	var finalRoots []string
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			abs = root
		}
		if !seen[abs] {
			seen[abs] = true
			// Check if directory exists
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				finalRoots = append(finalRoots, abs)
			}
		}
	}

	return finalRoots
}

func runExecute() {
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	roots := getSnippetRoots(cfg)
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr, "No snippet directories found. Run 'pb init' to create a starter layout.")
		os.Exit(1)
	}

	files, err := parser.DiscoverAndParse(roots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing snippets: %v\n", err)
		os.Exit(1)
	}

	var allSnippets []domain.Snippet
	for _, f := range files {
		allSnippets = append(allSnippets, f.Snippets...)
	}

	if len(allSnippets) == 0 {
		fmt.Fprintln(os.Stderr, "No snippets found in configured directories. Add markdown snippets with level-2 headings '##' and code blocks.")
		os.Exit(1)
	}

	m := tui.NewAppModel(cfg, allSnippets, files)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	appModel := finalModel.(*tui.AppModel)
	if appModel.Done {
		// Output the finalized snippet to stdout so it can be captured by terminal wrappers
		fmt.Print(appModel.FinalSnippet)
	}
}

func runInit() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	snippetsDir := filepath.Join(home, ".config", "peanutbutter", "snippets")
	if err := os.MkdirAll(snippetsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	snippetsFile := filepath.Join(snippetsDir, "snippets.md")
	if _, err := os.Stat(snippetsFile); err == nil {
		fmt.Printf("Snippets file already exists at %s\n", snippetsFile)
		return
	}

	starterContent := `---
name: Starter Snippets
tags: [starter, example]
---

## List directory contents

Show detailed directory listing.

` + "```sh" + `
ls -lsha <@path:?.>
` + "```" + `

## Curl JSON endpoint

Post a request body to a URL.

` + "```bash" + `
curl -X POST -H "Content-Type: application/json" -d '<@json_body>' <@url>
` + "```" + `
`

	if err := os.WriteFile(snippetsFile, []byte(starterContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing starter snippets: %v\n", err)
		os.Exit(1)
	}

	// Scaffold empty config file if not exists
	configDir := filepath.Join(home, ".config", "peanutbutter")
	configFile := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		starterConfig := `# Peanutbutter configuration TOML

[paths]
# snippets = ["/path/to/my/extra/snippets"]

[suggestion_commands]
timeout_ms = 2000
allow_commands = true
`
		_ = os.WriteFile(configFile, []byte(starterConfig), 0644)
	}

	fmt.Printf("Initialized successfully! Scaffolding completed at:\n- %s\n- %s\n", snippetsFile, configFile)
}

func runLint() {
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(2)
	}

	roots := getSnippetRoots(cfg)
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr, "No snippet roots configured.")
		os.Exit(1)
	}

	files, err := parser.DiscoverAndParse(roots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering/parsing files: %v\n", err)
		os.Exit(2)
	}

	findings := 0
	for _, f := range files {
		// Basic lint checks
		// 1. Check for duplicate names
		seenNames := make(map[string]bool)
		for _, snip := range f.Snippets {
			if seenNames[snip.Name] {
				fmt.Printf("%s: error: duplicate snippet name %q\n", f.Path, snip.Name)
				findings++
			}
			seenNames[snip.Name] = true

			// 2. Check for dependent variable reference ordering
			orderMap := make(map[string]int)
			for idx, p := range snip.Placeholders {
				orderMap[p.Name] = idx
			}

			// Parse command templates to check references
			for _, p := range snip.Placeholders {
				if p.IsCommand {
					// Extract <#name>
					re := regexp.MustCompile(`<#([a-zA-Z0-9_-]+)>`)
					matches := re.FindAllStringSubmatch(p.Command, -1)
					for _, match := range matches {
						refName := match[1]
						refIdx, exists := orderMap[refName]
						if !exists {
							fmt.Printf("%s: error inside %q: dependent variable reference %q is undeclared\n", f.Path, snip.Name, refName)
							findings++
						} else if refIdx >= orderMap[p.Name] {
							fmt.Printf("%s: error inside %q: forward variable reference to %q is forbidden\n", f.Path, snip.Name, refName)
							findings++
						}
					}
				}
			}
		}
	}

	if findings > 0 {
		fmt.Printf("Lint failed with %d findings.\n", findings)
		os.Exit(1)
	}

	fmt.Println("No lint findings.")
}
