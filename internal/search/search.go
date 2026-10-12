package search

import (
	"sort"
	"strings"

	"basin/internal/domain"
)

// SearchToken represents a parsed query filter.
type SearchToken struct {
	Operator string // "tag", "path", "name", "snippet", or "" for generic
	Value    string
}

// ParseQuery breaks a query string into operator tokens and generic text.
func ParseQuery(query string) []SearchToken {
	words := strings.Fields(query)
	var tokens []SearchToken

	for _, word := range words {
		if strings.HasPrefix(word, "tag:") {
			tokens = append(tokens, SearchToken{Operator: "tag", Value: strings.TrimPrefix(word, "tag:")})
		} else if strings.HasPrefix(word, "path:") {
			tokens = append(tokens, SearchToken{Operator: "path", Value: strings.TrimPrefix(word, "path:")})
		} else if strings.HasPrefix(word, "name:") {
			tokens = append(tokens, SearchToken{Operator: "name", Value: strings.TrimPrefix(word, "name:")})
		} else if strings.HasPrefix(word, "snippet:") {
			tokens = append(tokens, SearchToken{Operator: "snippet", Value: strings.TrimPrefix(word, "snippet:")})
		} else if strings.HasPrefix(word, "body:") {
			tokens = append(tokens, SearchToken{Operator: "snippet", Value: strings.TrimPrefix(word, "body:")})
		} else {
			tokens = append(tokens, SearchToken{Operator: "", Value: word})
		}
	}

	return tokens
}

// MatchSnippet checks if a snippet matches all search tokens.
func MatchSnippet(snip domain.Snippet, tokens []SearchToken) bool {
	if len(tokens) == 0 {
		return true
	}

	for _, tok := range tokens {
		val := strings.ToLower(tok.Value)
		switch tok.Operator {
		case "tag":
			matched := false
			for _, tag := range snip.Tags {
				if strings.Contains(strings.ToLower(tag), val) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		case "path":
			if !strings.Contains(strings.ToLower(snip.RelativePath), val) {
				return false
			}
		case "name":
			if !strings.Contains(strings.ToLower(snip.Name), val) {
				return false
			}
		case "snippet":
			if !strings.Contains(strings.ToLower(snip.Body), val) {
				return false
			}
		default:
			// Generic match: check name, description, tags, body, relative path
			matched := false
			if strings.Contains(strings.ToLower(snip.Name), val) {
				matched = true
			} else if strings.Contains(strings.ToLower(snip.Description), val) {
				matched = true
			} else if strings.Contains(strings.ToLower(snip.RelativePath), val) {
				matched = true
			} else if strings.Contains(strings.ToLower(snip.Body), val) {
				matched = true
			} else {
				for _, tag := range snip.Tags {
					if strings.Contains(strings.ToLower(tag), val) {
						matched = true
						break
					}
				}
			}
			if !matched {
				return false
			}
		}
	}
	return true
}

// FilterSnippets returns all snippets that match the search query.
func FilterSnippets(snippets []domain.Snippet, query string) []domain.Snippet {
	tokens := ParseQuery(query)
	var matched []domain.Snippet
	for _, snip := range snippets {
		if MatchSnippet(snip, tokens) {
			matched = append(matched, snip)
		}
	}
	return matched
}

type UseCountGetter func(snippetID string) int

// SortByUsage sorts snippets by usage count (descending), then by original order.
func SortByUsage(snippets []domain.Snippet, getUseCount UseCountGetter) []domain.Snippet {
	if len(snippets) == 0 {
		return snippets
	}
	
	sorted := make([]domain.Snippet, len(snippets))
	copy(sorted, snippets)
	
	sort.SliceStable(sorted, func(i, j int) bool {
		countI := getUseCount(sorted[i].ID)
		countJ := getUseCount(sorted[j].ID)
		return countI > countJ
	})
	
	return sorted
}
