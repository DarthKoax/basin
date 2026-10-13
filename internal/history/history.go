package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type SnippetUsage struct {
	SnippetID   string            `json:"snippet_id"`
	LastUsed    time.Time         `json:"last_used"`
	UseCount    int               `json:"use_count"`
	VariableMap map[string]string `json:"variable_map"`
}

type History struct {
	Usages map[string]*SnippetUsage `json:"usages"`
	mu     sync.RWMutex
}

func New() *History {
	return &History{
		Usages: make(map[string]*SnippetUsage),
	}
}

func Load() (*History, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return New(), nil
	}

	path := filepath.Join(home, ".config", "basin", "history.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, fmt.Errorf("reading history file: %w", err)
	}

	h := New()
	if err := json.Unmarshal(data, h); err != nil {
		return nil, fmt.Errorf("parsing history JSON: %w", err)
	}

	if h.Usages == nil {
		h.Usages = make(map[string]*SnippetUsage)
	}

	return h, nil
}

func (h *History) Save() error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	dir := filepath.Join(home, ".config", "basin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	path := filepath.Join(dir, "history.json")
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling history JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing history file: %w", err)
	}

	return nil
}

func (h *History) RecordUsage(snippetID string, variables map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	usage, exists := h.Usages[snippetID]
	if !exists {
		usage = &SnippetUsage{
			SnippetID: snippetID,
		}
		h.Usages[snippetID] = usage
	}

	usage.LastUsed = time.Now()
	usage.UseCount++
	
	if variables != nil {
		if usage.VariableMap == nil {
			usage.VariableMap = make(map[string]string)
		}
		for k, v := range variables {
			usage.VariableMap[k] = v
		}
	}
}

func (h *History) GetLastUsedValues(snippetID string) map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	usage, exists := h.Usages[snippetID]
	if !exists || usage.VariableMap == nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range usage.VariableMap {
		result[k] = v
	}
	return result
}

func (h *History) GetUseCount(snippetID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	usage, exists := h.Usages[snippetID]
	if !exists {
		return 0
	}
	return usage.UseCount
}

type SnippetRank struct {
	SnippetID string
	UseCount  int
	LastUsed  time.Time
}

func (h *History) GetRankedSnippets() []SnippetRank {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ranks := make([]SnippetRank, 0, len(h.Usages))
	for id, usage := range h.Usages {
		ranks = append(ranks, SnippetRank{
			SnippetID: id,
			UseCount:  usage.UseCount,
			LastUsed:  usage.LastUsed,
		})
	}

	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].UseCount != ranks[j].UseCount {
			return ranks[i].UseCount > ranks[j].UseCount
		}
		return ranks[i].LastUsed.After(ranks[j].LastUsed)
	})

	return ranks
}
