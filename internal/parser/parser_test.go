package parser

import (
	"strings"
	"testing"
)

func TestParseFile_Simple(t *testing.T) {
	mdContent := `---
name: Shell utilities
tags: [shell, files]
description: Common shell operations.
variables:
  environment:
    default: dev
    suggestions: [dev, stage, prod]
---

# Ignored Title

## List directory contents

This shows directory contents.

` + "```sh" + `
ls -la <@path:?.>
` + "```" + `

## Read base64 file

` + "```text" + `
Example preview block
` + "```" + `

` + "```bash" + `
base64 -d <@file:rg . --files>
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
	body := "echo <@input:?default> <@input:cmd> <@other>"
	placeholders := extractPlaceholders(body)

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
