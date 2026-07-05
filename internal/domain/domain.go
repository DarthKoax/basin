package domain

// VariableSpec represents input behaviors for free-form placeholders
// declared in the YAML frontmatter or global config.
type VariableSpec struct {
	Default     *string  `yaml:"default,omitempty" toml:"default,omitempty"`
	Suggestions []string `yaml:"suggestions,omitempty" toml:"suggestions,omitempty"`
	Command     string   `yaml:"command,omitempty" toml:"command,omitempty"`
}

// Boundaries defines custom delimiters for variable placeholders.
type Boundaries struct {
	Open  string `yaml:"open,omitempty" toml:"open,omitempty"`
	Close string `yaml:"close,omitempty" toml:"close,omitempty"`
}

// Placeholder represents a specific occurrence of a variable template inside the snippet body.
// E.g., <@container>, <@host:rg . --files>, <@port:?8080>.
type Placeholder struct {
	Name          string
	Raw           string
	DefaultValue  string
	Command       string
	IsDefault     bool
	IsCommand     bool
	SelectOptions []string // Non-nil when the placeholder is a select-type variable
	IsSelect      bool
}

// Snippet represents a single snippet declared under a "##" header in a Markdown file.
type Snippet struct {
	ID           string        // Stable identifier: relative_path + "#" + slug
	Name         string        // Heading text
	Description  string        // Description text between heading and executable body
	Body         string        // Body of first non-text code block
	Language     string        // Language tag of the executable code block
	Placeholders []Placeholder // Ordered and deduplicated placeholders extracted from body
	SourceFile   string        // Absolute file path
	RelativePath string        // Relative file path from the snippet root
	Tags         []string      // Tags inherited from file frontmatter
	Boundaries   Boundaries    // Custom variable boundary characters
}

// Frontmatter represents the YAML frontmatter metadata at the top of a Markdown snippet file.
type Frontmatter struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	Tags        []string                `yaml:"tags"`
	Variables   map[string]VariableSpec `yaml:"variables"`
	Boundaries  Boundaries              `yaml:"boundaries"`
}

// SnippetFile represents a parsed markdown file containing frontmatter and snippets.
type SnippetFile struct {
	Path         string
	RelativePath string
	Frontmatter  Frontmatter
	Snippets     []Snippet
}
