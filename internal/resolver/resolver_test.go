package resolver

import (
	"os"
	"testing"

	"peanutclone/internal/config"
	"peanutclone/internal/domain"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"sam's code", "'sam'\\''s code'"},
	}

	for _, tc := range tests {
		actual := ShellQuote(tc.input)
		if actual != tc.expected {
			t.Errorf("ShellQuote(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestResolveCommandTemplate(t *testing.T) {
	confirmed := map[string]string{
		"secret": "db-pass",
		"key":    "password",
		"query":  "select * from 'users'",
	}

	tests := []struct {
		input    string
		expected string
		err      bool
	}{
		{"kubectl get secret <#secret>", "kubectl get secret 'db-pass'", false},
		{"kubectl get secret <#secret> -o jsonpath='{.data.<#key:raw>}'", "kubectl get secret 'db-pass' -o jsonpath='{.data.password}'", false},
		{"echo \\<#secret>", "echo <#secret>", false},
		{"kubectl get secret <#missing>", "", true},
	}

	for _, tc := range tests {
		actual, err := ResolveCommandTemplate(tc.input, confirmed, domain.Boundaries{})
		if (err != nil) != tc.err {
			t.Errorf("ResolveCommandTemplate(%q) error state: %v, expected err: %v", tc.input, err, tc.err)
		}
		if err == nil && actual != tc.expected {
			t.Errorf("ResolveCommandTemplate(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestResolver_GetDefaultValue(t *testing.T) {
	cfg := config.DefaultConfig()
	devDefault := "dev"
	cfg.Variables["environment"] = domain.VariableSpec{
		Default: &devDefault,
	}

	fm := domain.Frontmatter{
		Variables: make(map[string]domain.VariableSpec),
	}
	stageDefault := "stage"
	fm.Variables["port"] = domain.VariableSpec{
		Default: &stageDefault,
	}

	snip := domain.Snippet{}
	r := NewResolver(cfg, snip, fm)

	// Test Inline
	inlinePlaceholder := domain.Placeholder{Name: "host", IsDefault: true, DefaultValue: "localhost"}
	if val := r.GetDefaultValue(inlinePlaceholder); val != "localhost" {
		t.Errorf("expected inline default localhost, got %q", val)
	}

	// Test Frontmatter
	fmPlaceholder := domain.Placeholder{Name: "port"}
	if val := r.GetDefaultValue(fmPlaceholder); val != "stage" {
		t.Errorf("expected frontmatter default stage, got %q", val)
	}

	// Test Config
	cfgPlaceholder := domain.Placeholder{Name: "environment"}
	if val := r.GetDefaultValue(cfgPlaceholder); val != "dev" {
		t.Errorf("expected config default dev, got %q", val)
	}
}

func TestResolver_GetSuggestions_Static(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Variables["region"] = domain.VariableSpec{
		Suggestions: []string{"us-east-1", "us-west-2"},
	}

	fm := domain.Frontmatter{
		Variables: make(map[string]domain.VariableSpec),
	}
	fm.Variables["env"] = domain.VariableSpec{
		Suggestions: []string{"dev", "prod"},
	}

	r := NewResolver(cfg, domain.Snippet{}, fm)

	// Test frontmatter static suggestions
	pEnv := domain.Placeholder{Name: "env"}
	suggs, err := r.GetSuggestions(pEnv, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggs) != 2 || suggs[0] != "dev" {
		t.Errorf("unexpected frontmatter suggestions: %v", suggs)
	}

	// Test config static suggestions
	pRegion := domain.Placeholder{Name: "region"}
	suggs, err = r.GetSuggestions(pRegion, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggs) != 2 || suggs[0] != "us-east-1" {
		t.Errorf("unexpected config suggestions: %v", suggs)
	}
}

func TestResolver_GetSuggestions_Command(t *testing.T) {
	cfg := config.DefaultConfig()
	r := NewResolver(cfg, domain.Snippet{}, domain.Frontmatter{})

	// Test command execution
	pCmd := domain.Placeholder{Name: "cmd_var", IsCommand: true, Command: "echo -e 'line1\\nline2'"}
	suggs, err := r.GetSuggestions(pCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error running command: %v", err)
	}

	if len(suggs) != 2 || suggs[0] != "line1" || suggs[1] != "line2" {
		t.Errorf("unexpected suggestions from command: %v", suggs)
	}
}

func TestResolver_GetSuggestions_Builtins(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pb-resolver-builtin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origWd)

	// Create a dummy file and folder
	if err := os.WriteFile("dummy.txt", []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}
	if err := os.Mkdir("dummy_dir", 0755); err != nil {
		t.Fatalf("failed to create dummy dir: %v", err)
	}

	r := NewResolver(config.DefaultConfig(), domain.Snippet{}, domain.Frontmatter{})

	// Test built-in 'file'
	sFiles, err := r.GetSuggestions(domain.Placeholder{Name: "file"}, nil)
	if err != nil {
		t.Fatalf("file builtin error: %v", err)
	}
	if len(sFiles) != 1 || sFiles[0] != "dummy.txt" {
		t.Errorf("unexpected file list: %v", sFiles)
	}

	// Test built-in 'directory'
	sDirs, err := r.GetSuggestions(domain.Placeholder{Name: "directory"}, nil)
	if err != nil {
		t.Fatalf("directory builtin error: %v", err)
	}
	if len(sDirs) != 1 || sDirs[0] != "dummy_dir" {
		t.Errorf("unexpected directory list: %v", sDirs)
	}
}

func TestResolver_GetSuggestions_SelectOptions(t *testing.T) {
	cfg := config.DefaultConfig()
	fm := domain.Frontmatter{
		Variables: make(map[string]domain.VariableSpec),
	}

	// Frontmatter defines suggestions for "method" but inline select takes precedence
	fm.Variables["method"] = domain.VariableSpec{
		Suggestions: []string{"override", "shouldNotSee"},
	}

	r := NewResolver(cfg, domain.Snippet{}, fm)

	// Inline select options should override everything else
	pMethod := domain.Placeholder{
		Name:          "method",
		IsSelect:      true,
		SelectOptions: []string{"GET", "POST", "PUT", "DELETE"},
		IsDefault:     true,
		DefaultValue:  "GET",
	}

	suggs, err := r.GetSuggestions(pMethod, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedOpts := []string{"GET", "POST", "PUT", "DELETE"}
	if len(suggs) != len(expectedOpts) {
		t.Errorf("expected %d suggestions, got %d: %v", len(expectedOpts), len(suggs), suggs)
	}
	for i, opt := range expectedOpts {
		if suggs[i] != opt {
			t.Errorf("suggestion[%d] = %q, expected %q", i, suggs[i], opt)
		}
	}

	// Default value should resolve to first option
	defVal := r.GetDefaultValue(pMethod)
	if defVal != "GET" {
		t.Errorf("expected default value 'GET', got %q", defVal)
	}
}

func TestResolver_GetSuggestions_SelectOverridesFrontmatter(t *testing.T) {
	cfg := config.DefaultConfig()
	fm := domain.Frontmatter{
		Variables: make(map[string]domain.VariableSpec),
	}
	cmdDefault := "ls -la"
	fm.Variables["action"] = domain.VariableSpec{
		Default:     &cmdDefault,
		Suggestions: []string{"fm1", "fm2"},
	}

	r := NewResolver(cfg, domain.Snippet{}, fm)

	pAction := domain.Placeholder{
		Name:          "action",
		IsSelect:      true,
		SelectOptions: []string{"inline1", "inline2", "inline3"},
		IsDefault:     true,
		DefaultValue:  "inline1",
	}

	suggs, err := r.GetSuggestions(pAction, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must return inline options, not frontmatter suggestions
	if len(suggs) != 3 || suggs[0] != "inline1" {
		t.Errorf("expected inline select options, got: %v", suggs)
	}

	defVal := r.GetDefaultValue(pAction)
	if defVal != "inline1" {
		t.Errorf("expected default 'inline1', got %q", defVal)
	}
}

func TestResolver_RenderSnippet(t *testing.T) {
	snip := domain.Snippet{
		Body: "curl -X <@method:?GET> -H <@headers:cmd> <@url>",
	}
	r := NewResolver(config.DefaultConfig(), snip, domain.Frontmatter{})

	confirmed := map[string]string{
		"method":  "POST",
		"headers": "Authorization: Bearer xyz",
		"url":     "https://example.com",
	}

	rendered := r.RenderSnippet(confirmed)
	expected := "curl -X POST -H Authorization: Bearer xyz https://example.com"
	if rendered != expected {
		t.Errorf("expected %q, got %q", expected, rendered)
	}

	// Test partial rendering -> env var fallback
	partialConfirmed := map[string]string{
		"method": "GET",
	}
	renderedPartial := r.RenderSnippet(partialConfirmed)
	expectedPartial := "curl -X GET -H $headers $url"
	if renderedPartial != expectedPartial {
		t.Errorf("expected %q, got %q", expectedPartial, renderedPartial)
	}
}

func TestResolver_GetSuggestions_SelectOverridesAllSources(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Variables["action"] = domain.VariableSpec{
		Suggestions: []string{"cfg1", "cfg2"},
	}

	fm := domain.Frontmatter{
		Variables: make(map[string]domain.VariableSpec),
	}
	cmdDefault := "ls -la"
	fm.Variables["action"] = domain.VariableSpec{
		Default:     &cmdDefault,
		Suggestions: []string{"fm1", "fm2"},
	}

	r := NewResolver(cfg, domain.Snippet{}, fm)

	pAction := domain.Placeholder{
		Name:          "action",
		IsSelect:      true,
		SelectOptions: []string{"inlineA", "inlineB"},
		IsDefault:     true,
		DefaultValue:  "inlineA",
	}

	suggs, err := r.GetSuggestions(pAction, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggs) != 2 || suggs[0] != "inlineA" {
		t.Errorf("expected inline select options to override all, got: %v", suggs)
	}
}

func TestResolver_RenderSnippet_SelectPlaceholder(t *testing.T) {
	snip := domain.Snippet{
		Body: "curl -X <$METHOD:?GET|POST> <$URL>",
		Placeholders: []domain.Placeholder{
			{
				Name:          "METHOD",
				Raw:           "<$METHOD:?GET|POST>",
				IsSelect:      true,
				SelectOptions: []string{"GET", "POST"},
				IsDefault:     true,
				DefaultValue:  "GET",
			},
			{
				Name: "URL",
				Raw:  "<$URL>",
			},
		},
	}
	r := NewResolver(config.DefaultConfig(), snip, domain.Frontmatter{})

	confirmed := map[string]string{
		"METHOD": "POST",
		"URL":    "https://api.example.com",
	}

	rendered := r.RenderSnippet(confirmed)
	expected := "curl -X POST https://api.example.com"
	if rendered != expected {
		t.Errorf("expected %q, got %q", expected, rendered)
	}

	unconfirmedURL := map[string]string{
		"METHOD": "GET",
	}

	renderedPartial := r.RenderSnippet(unconfirmedURL)
	expectedPartial := "curl -X GET $URL"
	if renderedPartial != expectedPartial {
		t.Errorf("expected %q, got %q", expectedPartial, renderedPartial)
	}
}

func TestResolver_RenderSnippet_SelectDefaultFallback(t *testing.T) {
	snip := domain.Snippet{
		Body: "curl -X <@method:?GET|POST> <$URL>",
		Placeholders: []domain.Placeholder{
			{
				Name:          "method",
				Raw:           "<@method:?GET|POST>",
				IsSelect:      true,
				SelectOptions: []string{"GET", "POST"},
				IsDefault:     true,
				DefaultValue:  "GET",
			},
			{
				Name: "URL",
				Raw:  "<$URL>",
			},
		},
	}
	r := NewResolver(config.DefaultConfig(), snip, domain.Frontmatter{})

	rendered := r.RenderSnippet(nil)
	expected := "curl -X GET $URL"
	if rendered != expected {
		t.Errorf("expected default fallback %q, got %q", expected, rendered)
	}
}

func TestResolver_RenderSnippet_CustomBoundaries(t *testing.T) {
	snip := domain.Snippet{
		Body: "echo {{@name:?world}} {{@greeting}}",
		Boundaries: domain.Boundaries{Open: "{{", Close: "}}"},
		Placeholders: []domain.Placeholder{
			{
				Name:         "name",
				Raw:          "{{@name:?world}}",
				IsDefault:    true,
				DefaultValue: "world",
			},
			{
				Name: "greeting",
				Raw:  "{{@greeting}}",
			},
		},
	}
	r := NewResolver(config.DefaultConfig(), snip, domain.Frontmatter{})

	confirmed := map[string]string{
		"name":     "Alice",
		"greeting": "Hello",
	}

	rendered := r.RenderSnippet(confirmed)
	expected := "echo Alice Hello"
	if rendered != expected {
		t.Errorf("expected %q, got %q", expected, rendered)
	}

	// Test default fallback
	renderedDefault := r.RenderSnippet(map[string]string{"greeting": "Hi"})
	expectedDefault := "echo world Hi"
	if renderedDefault != expectedDefault {
		t.Errorf("expected %q, got %q", expectedDefault, renderedDefault)
	}
}

func TestResolveCommandTemplate_CustomBoundaries(t *testing.T) {
	confirmed := map[string]string{
		"secret": "db-pass",
		"key":    "password",
	}

	boundaries := domain.Boundaries{Open: "{{", Close: "}}"}

	tests := []struct {
		input    string
		expected string
		err      bool
	}{
		{"kubectl get secret {{#secret}}", "kubectl get secret 'db-pass'", false},
		{"kubectl get secret {{#secret}} -o jsonpath='{.data.{{#key:raw}}}'", "kubectl get secret 'db-pass' -o jsonpath='{.data.password}'", false},
		{"echo \\{{#secret}}", "echo {{#secret}}", false},
		{"kubectl get secret {{#missing}}", "", true},
	}

	for _, tc := range tests {
		actual, err := ResolveCommandTemplate(tc.input, confirmed, boundaries)
		if (err != nil) != tc.err {
			t.Errorf("ResolveCommandTemplate(%q) error state: %v, expected err: %v", tc.input, err, tc.err)
		}
		if err == nil && actual != tc.expected {
			t.Errorf("ResolveCommandTemplate(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}
