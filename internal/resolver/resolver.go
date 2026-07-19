package resolver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"basin/internal/config"
	"basin/internal/domain"
)

var (
	dependentRegex  = regexp.MustCompile(`(\\)?<#([a-zA-Z0-9_-]+)(:raw)?>`)
	cmdSuggestionRe = regexp.MustCompile(`^\$\((.+)\)$`)
)

// isCmdSuggestion checks if a string is a $(...) command-backed suggestion.
func isCmdSuggestion(s string) (string, bool) {
	m := cmdSuggestionRe.FindStringSubmatch(s)
	if len(m) == 2 {
		return m[1], true
	}
	return "", false
}

// Resolver orchestrates default value and suggestion lookup for a snippet's placeholders.
type Resolver struct {
	Config      *config.Config
	Snippet     domain.Snippet
	Frontmatter domain.Frontmatter
}

// NewResolver creates a new Resolver for a given snippet.
func NewResolver(cfg *config.Config, snip domain.Snippet, fm domain.Frontmatter) *Resolver {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Resolver{
		Config:      cfg,
		Snippet:     snip,
		Frontmatter: fm,
	}
}

// ShellQuote wraps a string in single quotes and escapes existing single quotes.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ResolveCommandTemplate processes `<#name>` and `<#name:raw>` references in a command template.
func ResolveCommandTemplate(template string, confirmed map[string]string, boundaries domain.Boundaries) (string, error) {
	open := boundaries.Open
	close := boundaries.Close
	if open == "" {
		open = "<"
	}
	if close == "" {
		close = ">"
	}

	openEsc := regexp.QuoteMeta(open)
	closeEsc := regexp.QuoteMeta(close)

	pattern := `(\\)?` + openEsc + `#([a-zA-Z0-9_-]+)(:raw)?` + closeEsc
	depRe := regexp.MustCompile(pattern)

	var parseErr error
	res := depRe.ReplaceAllStringFunc(template, func(match string) string {
		submatches := depRe.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		escaped := submatches[1] != ""
		name := submatches[2]
		isRaw := submatches[3] == ":raw"

		if escaped {
			// Remove backslash, return literal <#name> or <#name:raw>
			return match[1:]
		}

		val, ok := confirmed[name]
		if !ok {
			parseErr = fmt.Errorf("dependent variable %q has no confirmed value yet", name)
			return match
		}

		if isRaw {
			return val
		}
		return ShellQuote(val)
	})

	if parseErr != nil {
		return "", parseErr
	}
	return res, nil
}

// GetDefaultValue returns the resolved default value for a placeholder.
// Priority: Placeholder fields (populated from frontmatter by parser) → Config.
func (r *Resolver) GetDefaultValue(p domain.Placeholder) string {
	// 1. Placeholder fields, populated from file-local frontmatter by the parser
	if p.IsDefault {
		return p.DefaultValue
	}

	// 2. Config-defined spec
	if cfgVar, ok := r.Config.Variables[p.Name]; ok && cfgVar.Default != nil {
		return *cfgVar.Default
	}

	return ""
}

// GetSuggestions resolves static or command-backed suggestions for a placeholder.
// Priority: Placeholder fields (populated from frontmatter by parser) → Config → Built-ins.
func (r *Resolver) GetSuggestions(p domain.Placeholder, confirmed map[string]string) ([]string, error) {
	// Priority 1: Placeholder fields, populated from file-local frontmatter by the parser
	var commandStr string
	var staticList []string
	hasCommand := false
	hasStatic := false

	if p.IsCommand {
		commandStr = p.Command
		hasCommand = true
	} else if p.IsSelect && len(p.SelectOptions) > 0 {
		// Check for $(...) command-backed suggestion
		if cmd, ok := isCmdSuggestion(p.SelectOptions[0]); ok {
			return r.runSuggestionCommand(cmd, confirmed)
		}
		return p.SelectOptions, nil
	}

	// Priority 2: Config-defined spec (if not already resolved from frontmatter)
	if !hasCommand && !hasStatic {
		if cfgVar, ok := r.Config.Variables[p.Name]; ok {
			if cfgVar.Command != "" {
				commandStr = cfgVar.Command
				hasCommand = true
			} else if len(cfgVar.Suggestions) > 0 {
				// Check for $(...) command-backed suggestion
				if cmd, ok := isCmdSuggestion(cfgVar.Suggestions[0]); ok {
					return r.runSuggestionCommand(cmd, confirmed)
				}
				staticList = cfgVar.Suggestions
				hasStatic = true
			}
		}
	}

	// Priority 3: Built-in suggestions (file / directory)
	if !hasCommand && !hasStatic {
		if p.Name == "file" {
			return listFilesAndDirs(false)
		} else if p.Name == "directory" {
			return listFilesAndDirs(true)
		}
		return nil, nil
	}

	if hasStatic {
		return staticList, nil
	}

	if hasCommand {
		return r.runSuggestionCommand(commandStr, confirmed)
	}

	return nil, nil
}

// runSuggestionCommand executes a bash command and returns its output lines as suggestions.
// Dependent variables (<#name>, <#name:raw>) are interpolated before execution.
func (r *Resolver) runSuggestionCommand(commandStr string, confirmed map[string]string) ([]string, error) {
	if !r.Config.SuggestionCommands.AllowCommands {
		return nil, nil
	}

	resolvedCmd, err := ResolveCommandTemplate(commandStr, confirmed, r.Snippet.Boundaries)
	if err != nil {
		return nil, fmt.Errorf("resolving command template: %w", err)
	}

	timeout := time.Duration(r.Config.SuggestionCommands.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", resolvedCmd)
	outputBytes, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("executing suggestion command %q: %w", resolvedCmd, err)
	}

	output := string(outputBytes)
	rawLines := strings.Split(output, "\n")
	var suggestions []string
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			suggestions = append(suggestions, line)
		}
	}
	return suggestions, nil
}

// RenderSnippet substitutes confirmed values into the snippet body.
func (r *Resolver) RenderSnippet(confirmed map[string]string) string {
	body := r.Snippet.Body

	open := r.Snippet.Boundaries.Open
	close := r.Snippet.Boundaries.Close
	if open == "" {
		open = "<"
	}
	if close == "" {
		close = ">"
	}

	openEsc := regexp.QuoteMeta(open)
	closeEsc := regexp.QuoteMeta(close)

	// Replace placeholders <#name> and <#name:raw> with confirmed values.
	depPattern := `(\\)?` + openEsc + `#([a-zA-Z0-9_-]+)(:raw)?` + closeEsc
	depRe := regexp.MustCompile(depPattern)
	body = depRe.ReplaceAllStringFunc(body, func(match string) string {
		submatches := depRe.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		escaped := submatches[1] != ""
		name := submatches[2]
		isRaw := submatches[3] == ":raw"

		if escaped {
			return match[1:]
		}

		val := confirmed[name]
		if isRaw {
			return val
		}
		return ShellQuote(val)
	})

	// Replace placeholders <$name> with either the provided value, a default, or an env var $NAME.
	phMap := make(map[string]domain.Placeholder)
	for _, ph := range r.Snippet.Placeholders {
		phMap[ph.Name] = ph
	}
	envPattern := openEsc + `\$([A-Za-z0-9_-]+)` + closeEsc
	envRe := regexp.MustCompile(envPattern)
	body = envRe.ReplaceAllStringFunc(body, func(m string) string {
		name := envRe.FindStringSubmatch(m)[1]
		if val, ok := confirmed[name]; ok && strings.TrimSpace(val) != "" {
			return val
		}
		if ph, ok := phMap[name]; ok && ph.IsDefault {
			return ph.DefaultValue
		}
		return fmt.Sprintf("$%s", name)
	})
	return body
}

func listFilesAndDirs(dirsOnly bool) ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue // Skip hidden files/directories
		}
		if dirsOnly {
			if entry.IsDir() {
				results = append(results, name)
			}
		} else {
			if !entry.IsDir() {
				results = append(results, name)
			}
		}
	}
	sort.Strings(results)
	return results, nil
}
