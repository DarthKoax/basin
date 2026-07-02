package search

import (
	"testing"

	"peanutclone/internal/domain"
)

func TestParseQuery(t *testing.T) {
	tokens := ParseQuery("tag:docker path:simple/snippets.md name:build generic-word")
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d", len(tokens))
	}

	if tokens[0].Operator != "tag" || tokens[0].Value != "docker" {
		t.Errorf("unexpected first token: %+v", tokens[0])
	}
	if tokens[1].Operator != "path" || tokens[1].Value != "simple/snippets.md" {
		t.Errorf("unexpected second token: %+v", tokens[1])
	}
	if tokens[2].Operator != "name" || tokens[2].Value != "build" {
		t.Errorf("unexpected third token: %+v", tokens[2])
	}
	if tokens[3].Operator != "" || tokens[3].Value != "generic-word" {
		t.Errorf("unexpected fourth token: %+v", tokens[3])
	}
}

func TestMatchSnippet(t *testing.T) {
	snip := domain.Snippet{
		Name:         "Build docker image",
		Description:  "Compile container files",
		Body:         "docker build -t my-app .",
		RelativePath: "nested/docker/snip.md",
		Tags:         []string{"docker", "build"},
	}

	tests := []struct {
		query string
		match bool
	}{
		{"", true},
		{"docker", true},
		{"compile", true},
		{"my-app", true},
		{"tag:docker", true},
		{"tag:missing", false},
		{"path:docker", true},
		{"path:simple", false},
		{"name:build", true},
		{"snippet:build", true},
		{"body:build", true},
		{"snippet:missing", false},
		{"tag:docker build image", true},
		{"tag:docker missing", false},
	}

	for _, tc := range tests {
		tokens := ParseQuery(tc.query)
		matched := MatchSnippet(snip, tokens)
		if matched != tc.match {
			t.Errorf("query %q: expected match=%t, got %t", tc.query, tc.match, matched)
		}
	}
}
