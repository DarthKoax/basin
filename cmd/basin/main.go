package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"basin/internal/config"
	"basin/internal/domain"
	"basin/internal/parser"
	"basin/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "basin",
	Short: "Basin - interactive terminal strand manager",
	Run: func(cmd *cobra.Command, args []string) {
		runExecute()
	},
}

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Run the interactive terminal strand selector",
	Run: func(cmd *cobra.Command, args []string) {
		runExecute()
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold starter strand directories and files",
	Run: func(cmd *cobra.Command, args []string) {
		runInit()
	},
}

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Check configured strand files for syntax and variable errors",
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

	roots = append(roots, cfg.Paths.Snippets...)

	envPath := os.Getenv("BASIN_PATH")
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

	home, err := os.UserHomeDir()
	if err == nil {
		xdgPath := filepath.Join(home, ".config", "basin", "strands")
		roots = append(roots, xdgPath)
	}

	seen := make(map[string]bool)
	var finalRoots []string
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			abs = root
		}
		if !seen[abs] {
			seen[abs] = true
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
		fmt.Fprintln(os.Stderr, "No strand directories found. Run 'basin init' to create a starter layout.")
		os.Exit(1)
	}

	files, err := parser.DiscoverAndParse(roots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing strands: %v\n", err)
		os.Exit(1)
	}

	var allSnippets []domain.Snippet
	for _, f := range files {
		allSnippets = append(allSnippets, f.Snippets...)
	}

	if len(allSnippets) == 0 {
		fmt.Fprintln(os.Stderr, "No strands found in configured directories. Add markdown strands with level-2 headings '##' and code blocks.")
		os.Exit(1)
	}

	m := tui.NewAppModel(cfg, roots, allSnippets, files)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	appModel := finalModel.(*tui.AppModel)
	if appModel.Done {
		fmt.Print(appModel.FinalSnippet)
	}
}

func runInit() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	strandsDir := filepath.Join(home, ".config", "basin", "strands")
	if err := os.MkdirAll(strandsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	strandsFile := filepath.Join(strandsDir, "strands.md")
	if _, err := os.Stat(strandsFile); err == nil {
		fmt.Printf("Strands file already exists at %s\n", strandsFile)
		return
	}

	starterContent := `---
name: Starter Strands
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

	if err := os.WriteFile(strandsFile, []byte(starterContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing starter strands: %v\n", err)
		os.Exit(1)
	}

	configDir := filepath.Join(home, ".config", "basin")
	configFile := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		starterConfig := `# Basin configuration TOML

[paths]
# strands = ["/path/to/my/extra/strands"]

[suggestion_commands]
timeout_ms = 2000
allow_commands = true
`
		_ = os.WriteFile(configFile, []byte(starterConfig), 0644)
	}

	fmt.Printf("Initialized successfully! Scaffolding completed at:\n- %s\n- %s\n", strandsFile, configFile)
}

func runLint() {
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(2)
	}

	roots := getSnippetRoots(cfg)
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr, "No strand roots configured.")
		os.Exit(1)
	}

	files, err := parser.DiscoverAndParse(roots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering/parsing files: %v\n", err)
		os.Exit(2)
	}

	findings := 0
	for _, f := range files {
		seenNames := make(map[string]bool)
		for _, snip := range f.Snippets {
			if seenNames[snip.Name] {
				fmt.Printf("%s: error: duplicate strand name %q\n", f.Path, snip.Name)
				findings++
			}
			seenNames[snip.Name] = true

			orderMap := make(map[string]int)
			for idx, p := range snip.Placeholders {
				orderMap[p.Name] = idx
			}

			for _, p := range snip.Placeholders {
				if p.IsCommand {
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
