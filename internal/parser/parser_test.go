package parser

import (
	"strings"
	"testing"

	"basin/internal/domain"
)

func strPtr(s string) *string { return &s }

func TestParseFile_Simple(t *testing.T) {
	mdContent := `---
name: Shell utilities
tags: [shell, files]
description: Common shell operations.
variables:
  environment:
    default: dev
    suggestions: [dev, stage, prod]
  path:
    default: "."
  file:
    command: "rg . --files"
---

# Ignored Title

## List directory contents

This shows directory contents.

` + "```sh" + `
ls -la <@path>
` + "```" + `

## Read base64 file

` + "```text" + `
Example preview block
` + "```" + `

` + "```bash" + `
base64 -d <@file>
` + "```" + `
`

	sf, err := ParseFile("simple.md", ".", strings.NewReader(mdContent))
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	if sf.Frontmatter.Name != "Shell utilities" {
		t.Errorf("unexpected frontmatter name: %s", sf.Frontmatter.Name)
	}

	if len(sf.Frontmatter.Tags) != 2 || sf.Frontmatter.Tags[0] != "shell" {
		t.Errorf("unexpected tags: %v", sf.Frontmatter.Tags)
	}

	envVar, exists := sf.Frontmatter.Variables["environment"]
	if !exists {
		t.Fatal("expected variable 'environment' to exist")
	}
	if envVar.Default == nil || *envVar.Default != "dev" {
		t.Errorf("unexpected variable default: %v", envVar.Default)
	}

	if len(sf.Snippets) != 2 {
		t.Fatalf("expected 2 snippets, got %d", len(sf.Snippets))
	}

	s1 := sf.Snippets[0]
	if s1.Name != "List directory contents" {
		t.Errorf("unexpected name: %s", s1.Name)
	}
	if s1.Description != "This shows directory contents." {
		t.Errorf("unexpected description: %q", s1.Description)
	}
	if len(s1.Placeholders) != 1 || s1.Placeholders[0].Name != "path" || !s1.Placeholders[0].IsDefault || s1.Placeholders[0].DefaultValue != "." {
		t.Errorf("unexpected placeholders in s1: %v", s1.Placeholders)
	}

	s2 := sf.Snippets[1]
	if s2.Name != "Read base64 file" {
		t.Errorf("unexpected name: %s", s2.Name)
	}
	if !strings.Contains(s2.Description, "Example preview block") {
		t.Errorf("expected description to contain preview text, got: %q", s2.Description)
	}
	if len(s2.Placeholders) != 1 || s2.Placeholders[0].Name != "file" || !s2.Placeholders[0].IsCommand || s2.Placeholders[0].Command != "rg . --files" {
		t.Errorf("unexpected placeholders in s2: %v", s2.Placeholders)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"List directory contents", "list-directory-contents"},
		{"Create a directory and navigate into it", "create-a-directory-and-navigate-into-it"},
		{"Echo", "echo"},
		{"What is this?!", "what-is-this"},
		{"git_branch", "git-branch"},
	}

	for _, tc := range tests {
		actual := Slugify(tc.input)
		if actual != tc.expected {
			t.Errorf("slugify(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestExtractPlaceholders_Deduplicate(t *testing.T) {
	body := "echo <@input> <@input> <@other>"
	variables := map[string]domain.VariableSpec{
		"input": {Default: strPtr("default")},
	}
	placeholders := extractPlaceholders(body, domain.Boundaries{}, variables)

	if len(placeholders) != 2 {
		t.Fatalf("expected 2 unique placeholders, got %d", len(placeholders))
	}

	if placeholders[0].Name != "input" || !placeholders[0].IsDefault || placeholders[0].DefaultValue != "default" {
		t.Errorf("unexpected first placeholder: %+v", placeholders[0])
	}

	if placeholders[1].Name != "other" || placeholders[1].IsDefault || placeholders[1].IsCommand {
		t.Errorf("unexpected second placeholder: %+v", placeholders[1])
	}
}

func TestExtractPlaceholders_PipeSeparatedOptions(t *testing.T) {
	body := "curl -X <@method> <@url>"
	variables := map[string]domain.VariableSpec{
		"method": {
			Default:     strPtr("GET"),
			Suggestions: []string{"GET", "POST", "PUT", "DELETE"},
		},
		"url": {Default: strPtr(".")},
	}

	placeholders := extractPlaceholders(body, domain.Boundaries{}, variables)

	if len(placeholders) != 2 {
		t.Fatalf("expected 2 placeholders, got %d", len(placeholders))
	}

	// First placeholder should be a select with pipe-separated options
	m := placeholders[0]
	if m.Name != "method" {
		t.Errorf("expected name 'method', got %q", m.Name)
	}
	if !m.IsSelect {
		t.Error("expected IsSelect to be true")
	}
	if !m.IsDefault {
		t.Error("expected IsDefault to be true for select placeholder")
	}
	if m.DefaultValue != "GET" {
		t.Errorf("expected default value 'GET', got %q", m.DefaultValue)
	}
	expectedOpts := []string{"GET", "POST", "PUT", "DELETE"}
	if len(m.SelectOptions) != len(expectedOpts) {
		t.Fatalf("expected %d select options, got %d", len(expectedOpts), len(m.SelectOptions))
	}
	for i, opt := range expectedOpts {
		if m.SelectOptions[i] != opt {
			t.Errorf("selectOptions[%d] = %q, expected %q", i, m.SelectOptions[i], opt)
		}
	}

	// Second placeholder should be a regular default (no pipes)
	s := placeholders[1]
	if s.Name != "url" || !s.IsDefault || s.DefaultValue != "." {
		t.Errorf("unexpected second placeholder: %+v", s)
	}
	if s.IsSelect {
		t.Error("expected IsSelect to be false for regular default placeholder")
	}
	if len(s.SelectOptions) != 0 {
		t.Errorf("expected no select options, got %v", s.SelectOptions)
	}
}

func TestExtractPlaceholders_SelectAndCommandMixed(t *testing.T) {
	body := "docker run <@image> -p <@port> --volume <@path>"
	variables := map[string]domain.VariableSpec{
		"image": {
			Default:     strPtr("nginx"),
			Suggestions: []string{"nginx", "redis"},
		},
		"port":  {Default: strPtr(".")},
		"path":  {Command: "npx dir-select"},
	}

	placeholders := extractPlaceholders(body, domain.Boundaries{}, variables)

	if len(placeholders) != 3 {
		t.Fatalf("expected 3 placeholders, got %d", len(placeholders))
	}

	// First should be select
	image := placeholders[0]
	if !image.IsSelect || len(image.SelectOptions) != 2 {
		t.Errorf("placeholder 'image' should be select with 2 options, got: %+v", image)
	}

	// Second should be default
	port := placeholders[1]
	if port.IsSelect || port.IsCommand || !port.IsDefault {
		t.Errorf("placeholder 'port' should be default only, got: %+v", port)
	}

	// Third should be command
	pathP := placeholders[2]
	if pathP.IsSelect || pathP.IsDefault || !pathP.IsCommand {
		t.Errorf("placeholder 'path' should be command, got: %+v", pathP)
	}
}

func TestExtractPlaceholders_SelectWithDollarPrefix(t *testing.T) {
	body := "curl -X <$METHOD> <$URL>"
	variables := map[string]domain.VariableSpec{
		"METHOD": {
			Default:     strPtr("GET"),
			Suggestions: []string{"GET", "POST"},
		},
	}
	placeholders := extractPlaceholders(body, domain.Boundaries{}, variables)

	if len(placeholders) != 2 {
		t.Fatalf("expected 2 placeholders, got %d", len(placeholders))
	}

	m := placeholders[0]
	if m.Name != "METHOD" {
		t.Errorf("expected name 'METHOD', got %q", m.Name)
	}
	if !m.IsSelect || len(m.SelectOptions) != 2 {
		t.Errorf("dollar-prefix select not parsed: %+v", m)
	}

	u := placeholders[1]
	if u.Name != "URL" {
		t.Errorf("expected name 'URL', got %q", u.Name)
	}
}

func TestExtractPlaceholders_SelectSinglePipeElementNotSelect(t *testing.T) {
	body := "<@host>"
	variables := map[string]domain.VariableSpec{
		"host": {Default: strPtr("localhost")},
	}
	placeholders := extractPlaceholders(body, domain.Boundaries{}, variables)

	if len(placeholders) != 1 {
		t.Fatalf("expected 1 placeholder, got %d", len(placeholders))
	}

	p := placeholders[0]
	if p.IsSelect {
		t.Error("single-element default should not be marked as select")
	}
	if p.DefaultValue != "localhost" {
		t.Errorf("expected 'localhost', got %q", p.DefaultValue)
	}
}

func TestExtractPlaceholders_SelectWithSpacesInOptions(t *testing.T) {
	body := "<@env>"
	variables := map[string]domain.VariableSpec{
		"env": {
			Default:     strPtr("development"),
			Suggestions: []string{"development", "staging", "production"},
		},
	}
	placeholders := extractPlaceholders(body, domain.Boundaries{}, variables)

	if len(placeholders) != 1 {
		t.Fatalf("expected 1 placeholder, got %d", len(placeholders))
	}

	p := placeholders[0]
	if !p.IsSelect {
		t.Fatal("expected select type")
	}
	expectedOpts := []string{"development", "staging", "production"}
	if len(p.SelectOptions) != 3 {
		t.Errorf("expected 3 options, got %d: %v", len(p.SelectOptions), p.SelectOptions)
	}
	for i, opt := range expectedOpts {
		if p.SelectOptions[i] != opt {
			t.Errorf("option[%d] = %q, expected %q", i, p.SelectOptions[i], opt)
		}
	}
}

func TestParseFile_SelectPlaceholderInSnippet(t *testing.T) {
	mdContent := `---
name: HTTP snippets
tags: [http, curl]
variables:
  CURL_METHOD:
    default: GET
    suggestions: [GET, POST, PUT, DELETE]
---

## HTTP request method toggle

Toggle the method for a request.

` + "```sh" + `
curl -X <$CURL_METHOD> <$CURL_URL>
` + "```" + `
`

	sf, err := ParseFile("http.md", ".", strings.NewReader(mdContent))
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	if len(sf.Snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(sf.Snippets))
	}

	snip := sf.Snippets[0]
	if snip.Name != "HTTP request method toggle" {
		t.Errorf("unexpected name: %s", snip.Name)
	}

	if len(snip.Placeholders) != 2 {
		t.Fatalf("expected 2 placeholders, got %d", len(snip.Placeholders))
	}

	method := snip.Placeholders[0]
	if method.Name != "CURL_METHOD" {
		t.Errorf("expected placeholder name CURL_METHOD, got %q", method.Name)
	}
	if !method.IsSelect {
		t.Fatal("expected IsSelect to be true")
	}
	expectedOpts := []string{"GET", "POST", "PUT", "DELETE"}
	if len(method.SelectOptions) != 4 {
		t.Errorf("expected 4 select options, got %d: %v", len(method.SelectOptions), method.SelectOptions)
	}
	for i, opt := range expectedOpts {
		if method.SelectOptions[i] != opt {
			t.Errorf("option[%d] = %q, expected %q", i, method.SelectOptions[i], opt)
		}
	}

	url := snip.Placeholders[1]
	if url.Name != "CURL_URL" || url.IsSelect {
		t.Errorf("CURL_URL should not be select: %+v", url)
	}
}

func TestParseFile_CustomBoundaries(t *testing.T) {
	mdContent := `---
name: Custom boundaries
tags: [test]
boundaries:
  open: "{{"
  close: "}}"
variables:
  name:
    default: world
  greeting:
    command: "hello"
---

## Test custom boundaries

Test snippet with custom delimiters.

` + "```sh" + `
echo {{@name}} {{@greeting}}
` + "```" + `
`

	sf, err := ParseFile("custom.md", ".", strings.NewReader(mdContent))
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	if sf.Frontmatter.Boundaries.Open != "{{" {
		t.Errorf("expected open boundary '{{', got %q", sf.Frontmatter.Boundaries.Open)
	}
	if sf.Frontmatter.Boundaries.Close != "}}" {
		t.Errorf("expected close boundary '}}', got %q", sf.Frontmatter.Boundaries.Close)
	}

	if len(sf.Snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(sf.Snippets))
	}

	snip := sf.Snippets[0]
	if snip.Boundaries.Open != "{{" || snip.Boundaries.Close != "}}" {
		t.Errorf("snippet should have custom boundaries: %+v", snip.Boundaries)
	}

	if len(snip.Placeholders) != 2 {
		t.Fatalf("expected 2 placeholders, got %d", len(snip.Placeholders))
	}

	name := snip.Placeholders[0]
	if name.Name != "name" || !name.IsDefault || name.DefaultValue != "world" {
		t.Errorf("unexpected first placeholder: %+v", name)
	}

	greeting := snip.Placeholders[1]
	if greeting.Name != "greeting" || !greeting.IsCommand || greeting.Command != "hello" {
		t.Errorf("unexpected second placeholder: %+v", greeting)
	}
}

func TestExtractPlaceholders_CustomBoundaries(t *testing.T) {
	body := "echo {{@name}} {{@greeting}}"
	boundaries := domain.Boundaries{Open: "{{", Close: "}}"}
	variables := map[string]domain.VariableSpec{
		"name":     {Default: strPtr("world")},
		"greeting": {Command: "hello"},
	}
	placeholders := extractPlaceholders(body, boundaries, variables)

	if len(placeholders) != 2 {
		t.Fatalf("expected 2 placeholders, got %d", len(placeholders))
	}

	if placeholders[0].Name != "name" || !placeholders[0].IsDefault || placeholders[0].DefaultValue != "world" {
		t.Errorf("unexpected first placeholder: %+v", placeholders[0])
	}

	if placeholders[1].Name != "greeting" || !placeholders[1].IsCommand || placeholders[1].Command != "hello" {
		t.Errorf("unexpected second placeholder: %+v", placeholders[1])
	}
}

func TestExtractPlaceholders_CustomBoundariesWithSelect(t *testing.T) {
	body := "{{@method}}"
	boundaries := domain.Boundaries{Open: "{{", Close: "}}"}
	variables := map[string]domain.VariableSpec{
		"method": {
			Default:     strPtr("GET"),
			Suggestions: []string{"GET", "POST", "PUT"},
		},
	}
	placeholders := extractPlaceholders(body, boundaries, variables)

	if len(placeholders) != 1 {
		t.Fatalf("expected 1 placeholder, got %d", len(placeholders))
	}

	p := placeholders[0]
	if !p.IsSelect || len(p.SelectOptions) != 3 {
		t.Errorf("expected select with 3 options, got: %+v", p)
	}
}
