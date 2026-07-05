package parser

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"basin/internal/domain"
)

var defaultBoundaries = domain.Boundaries{Open: "<", Close: ">"}

// Slugify converts a string name into a URL/id-friendly slug.
func Slugify(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			lastDash = false
		} else {
			if !lastDash && sb.Len() > 0 {
				sb.WriteRune('-')
				lastDash = true
			}
		}
	}
	res := sb.String()
	return strings.TrimSuffix(res, "-")
}

// buildPlaceholderRegex creates a regex pattern for the given boundary characters.
func buildPlaceholderRegex(open, close string) *regexp.Regexp {
	openEsc := regexp.QuoteMeta(open)
	closeEsc := regexp.QuoteMeta(close)
	pattern := openEsc + `[@$]([a-zA-Z0-9_-]+)(?::(\?[^` + closeEsc + `]*|[^` + closeEsc + `]+))?` + closeEsc
	return regexp.MustCompile(pattern)
}

// ParseFile parses a markdown file, reading optional frontmatter and extracting snippets.
func ParseFile(path string, rootDir string, reader io.Reader) (*domain.SnippetFile, error) {
	relPath, err := filepath.Rel(rootDir, path)
	if err != nil {
		relPath = path
	}
	// Normalise backslashes to forward slashes for cross-platform stability in IDs
	relPath = filepath.ToSlash(relPath)

	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	snippetFile := &domain.SnippetFile{
		Path:         path,
		RelativePath: relPath,
	}

	var frontmatterLines []string
	var startIdx int
	// Parse YAML frontmatter
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		var foundClosing bool
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				startIdx = i + 1
				foundClosing = true
				break
			}
			frontmatterLines = append(frontmatterLines, lines[i])
		}
		if foundClosing && len(frontmatterLines) > 0 {
			var fm domain.Frontmatter
			fmText := strings.Join(frontmatterLines, "\n")
			if err := yaml.Unmarshal([]byte(fmText), &fm); err == nil {
				snippetFile.Frontmatter = fm
			}
		}
	}

	// Parsing state machine
	type parsedRawSnippet struct {
		name      string
		descLines []string
		bodyLines []string
		language  string
		hasBody   bool
	}

	var rawSnippets []*parsedRawSnippet
	var current *parsedRawSnippet

	inCodeBlock := false
	var codeBlockLines []string
	var currentLang string

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]

		// Check code block fences
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCodeBlock {
				// Closing fence
				inCodeBlock = false
				if current != nil {
					// Check if this was the executable body
					if !current.hasBody && currentLang != "text" {
						current.bodyLines = codeBlockLines
						current.language = currentLang
						current.hasBody = true
					} else {
						// treated as description/preview
						current.descLines = append(current.descLines, "```"+currentLang)
						current.descLines = append(current.descLines, codeBlockLines...)
						current.descLines = append(current.descLines, "```")
					}
				}
				codeBlockLines = nil
			} else {
				// Opening fence
				inCodeBlock = true
				trimmed := strings.TrimSpace(line)
				currentLang = strings.TrimPrefix(trimmed, "```")
				currentLang = strings.TrimSpace(currentLang)
				codeBlockLines = nil
			}
			continue
		}

		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}

		// Check level-2 headers outside code blocks
		if strings.HasPrefix(line, "## ") {
			// Start new snippet
			name := strings.TrimPrefix(line, "## ")
			name = strings.TrimSpace(name)
			current = &parsedRawSnippet{
				name: name,
			}
			rawSnippets = append(rawSnippets, current)
			continue
		}

		// Append normal text to description
		if current != nil && !current.hasBody {
			current.descLines = append(current.descLines, line)
		}
	}

	// Build the actual snippets
	for _, raw := range rawSnippets {
		if !raw.hasBody {
			continue // Skip snippets that do not have an executable body
		}

		bodyText := strings.Join(raw.bodyLines, "\n")
		descText := strings.TrimSpace(strings.Join(raw.descLines, "\n"))

		placeholders := extractPlaceholders(bodyText, snippetFile.Frontmatter.Boundaries)

		slug := Slugify(raw.name)
		id := relPath + "#" + slug

		snippet := domain.Snippet{
			ID:           id,
			Name:         raw.name,
			Description:  descText,
			Body:         bodyText,
			Language:     raw.language,
			Placeholders: placeholders,
			SourceFile:   path,
			RelativePath: relPath,
			Tags:         snippetFile.Frontmatter.Tags,
			Boundaries:   snippetFile.Frontmatter.Boundaries,
		}
		snippetFile.Snippets = append(snippetFile.Snippets, snippet)
	}

	return snippetFile, nil
}

// ParseFilePath reads and parses a markdown snippet file by path.
func ParseFilePath(path string, rootDir string) (*domain.SnippetFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ParseFile(path, rootDir, file)
}

// DiscoverAndParse discovers and parses all markdown files in the given snippet roots.
func DiscoverAndParse(roots []string) ([]domain.SnippetFile, error) {
	var files []domain.SnippetFile
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				sf, err := ParseFilePath(path, root)
				if err == nil {
					files = append(files, *sf)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func extractPlaceholders(body string, boundaries domain.Boundaries) []domain.Placeholder {
	open := boundaries.Open
	close := boundaries.Close
	if open == "" {
		open = defaultBoundaries.Open
	}
	if close == "" {
		close = defaultBoundaries.Close
	}

	re := buildPlaceholderRegex(open, close)
	matches := re.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	var placeholders []domain.Placeholder
	seen := make(map[string]bool)

	for _, match := range matches {
		raw := match[0]
		name := match[1]
		source := match[2]

		var defVal string
		var cmd string
		var isDef bool
		var isCmd bool
		var isSelect bool
		var selectOptions []string

		if source != "" {
			if strings.HasPrefix(source, "?") {
				valuePart := strings.TrimPrefix(source, "?")
				parts := strings.Split(valuePart, "|")
				if len(parts) > 1 {
					isDef = true
					isSelect = true
					defVal = parts[0]
					for _, p := range parts {
						selectOptions = append(selectOptions, p)
					}
				} else {
					isDef = true
					defVal = valuePart
				}
			} else {
				isCmd = true
				cmd = source
			}
		}

		if seen[name] {
			continue // deduplicate by name, keeping first occurrence
		}
		seen[name] = true

		placeholders = append(placeholders, domain.Placeholder{
			Name:          name,
			Raw:           raw,
			DefaultValue:  defVal,
			Command:       cmd,
			IsDefault:     isDef,
			IsCommand:     isCmd,
			IsSelect:      isSelect,
			SelectOptions: selectOptions,
		})
	}

	return placeholders
}
