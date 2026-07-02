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
		actual, err := ResolveCommandTemplate(tc.input, confirmed)
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

	// Test partial rendering
	partialConfirmed := map[string]string{
		"method": "GET",
	}
	renderedPartial := r.RenderSnippet(partialConfirmed)
	expectedPartial := "curl -X GET -H <@headers:cmd> <@url>"
	if renderedPartial != expectedPartial {
		t.Errorf("expected %q, got %q", expectedPartial, renderedPartial)
	}
}
