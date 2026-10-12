package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("New() returned nil")
	}
	if h.Usages == nil {
		t.Error("New() did not initialize Usages map")
	}
	if len(h.Usages) != 0 {
		t.Errorf("New() Usages map should be empty, got %d entries", len(h.Usages))
	}
}

func TestRecordUsage(t *testing.T) {
	h := New()
	
	vars := map[string]string{
		"container": "my-app",
		"host":      "localhost",
	}
	
	h.RecordUsage("test-snippet-1", vars)
	
	usage := h.Usages["test-snippet-1"]
	if usage == nil {
		t.Fatal("RecordUsage did not create usage entry")
	}
	
	if usage.SnippetID != "test-snippet-1" {
		t.Errorf("Expected SnippetID 'test-snippet-1', got %q", usage.SnippetID)
	}
	
	if usage.UseCount != 1 {
		t.Errorf("Expected UseCount 1, got %d", usage.UseCount)
	}
	
	if usage.VariableMap["container"] != "my-app" {
		t.Errorf("Expected container='my-app', got %q", usage.VariableMap["container"])
	}
	
	if usage.VariableMap["host"] != "localhost" {
		t.Errorf("Expected host='localhost', got %q", usage.VariableMap["host"])
	}
}

func TestRecordUsage_MultipleUses(t *testing.T) {
	h := New()
	
	h.RecordUsage("test-snippet-1", map[string]string{"var1": "value1"})
	h.RecordUsage("test-snippet-1", map[string]string{"var1": "value2", "var2": "value3"})
	
	usage := h.Usages["test-snippet-1"]
	if usage.UseCount != 2 {
		t.Errorf("Expected UseCount 2, got %d", usage.UseCount)
	}
	
	if usage.VariableMap["var1"] != "value2" {
		t.Errorf("Expected var1='value2' (latest), got %q", usage.VariableMap["var1"])
	}
	
	if usage.VariableMap["var2"] != "value3" {
		t.Errorf("Expected var2='value3', got %q", usage.VariableMap["var2"])
	}
}

func TestGetLastUsedValues(t *testing.T) {
	h := New()
	
	vars := map[string]string{
		"container": "my-app",
		"host":      "localhost",
	}
	
	h.RecordUsage("test-snippet-1", vars)
	
	retrieved := h.GetLastUsedValues("test-snippet-1")
	if retrieved == nil {
		t.Fatal("GetLastUsedValues returned nil")
	}
	
	if retrieved["container"] != "my-app" {
		t.Errorf("Expected container='my-app', got %q", retrieved["container"])
	}
	
	if retrieved["host"] != "localhost" {
		t.Errorf("Expected host='localhost', got %q", retrieved["host"])
	}
}

func TestGetLastUsedValues_NotFound(t *testing.T) {
	h := New()
	
	retrieved := h.GetLastUsedValues("nonexistent")
	if retrieved != nil {
		t.Errorf("Expected nil for nonexistent snippet, got %v", retrieved)
	}
}

func TestGetUseCount(t *testing.T) {
	h := New()
	
	if count := h.GetUseCount("test-snippet-1"); count != 0 {
		t.Errorf("Expected UseCount 0 for new snippet, got %d", count)
	}
	
	h.RecordUsage("test-snippet-1", nil)
	if count := h.GetUseCount("test-snippet-1"); count != 1 {
		t.Errorf("Expected UseCount 1, got %d", count)
	}
	
	h.RecordUsage("test-snippet-1", nil)
	if count := h.GetUseCount("test-snippet-1"); count != 2 {
		t.Errorf("Expected UseCount 2, got %d", count)
	}
}

func TestGetRankedSnippets(t *testing.T) {
	h := New()
	
	h.RecordUsage("snippet-a", nil)
	h.RecordUsage("snippet-b", nil)
	h.RecordUsage("snippet-b", nil)
	h.RecordUsage("snippet-c", nil)
	h.RecordUsage("snippet-c", nil)
	h.RecordUsage("snippet-c", nil)
	
	ranked := h.GetRankedSnippets()
	
	if len(ranked) != 3 {
		t.Fatalf("Expected 3 ranked snippets, got %d", len(ranked))
	}
	
	if ranked[0].SnippetID != "snippet-c" || ranked[0].UseCount != 3 {
		t.Errorf("Expected first rank to be snippet-c with count 3, got %s with count %d", 
			ranked[0].SnippetID, ranked[0].UseCount)
	}
	
	if ranked[1].SnippetID != "snippet-b" || ranked[1].UseCount != 2 {
		t.Errorf("Expected second rank to be snippet-b with count 2, got %s with count %d", 
			ranked[1].SnippetID, ranked[1].UseCount)
	}
	
	if ranked[2].SnippetID != "snippet-a" || ranked[2].UseCount != 1 {
		t.Errorf("Expected third rank to be snippet-a with count 1, got %s with count %d", 
			ranked[2].SnippetID, ranked[2].UseCount)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	h1 := New()
	h1.RecordUsage("snippet-1", map[string]string{"var1": "value1"})
	h1.RecordUsage("snippet-2", map[string]string{"var2": "value2"})
	
	if err := h1.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	
	h2, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	
	if len(h2.Usages) != 2 {
		t.Errorf("Expected 2 usages after load, got %d", len(h2.Usages))
	}
	
	usage1 := h2.Usages["snippet-1"]
	if usage1 == nil {
		t.Fatal("snippet-1 not found after load")
	}
	if usage1.VariableMap["var1"] != "value1" {
		t.Errorf("Expected var1='value1' after load, got %q", usage1.VariableMap["var1"])
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	h, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error for nonexistent file: %v", err)
	}
	
	if h == nil {
		t.Fatal("Load() returned nil")
	}
	
	if len(h.Usages) != 0 {
		t.Errorf("Expected empty usages map, got %d entries", len(h.Usages))
	}
}

func TestLastUsedTimestamp(t *testing.T) {
	h := New()
	
	before := time.Now()
	h.RecordUsage("snippet-1", nil)
	after := time.Now()
	
	usage := h.Usages["snippet-1"]
	if usage.LastUsed.Before(before) || usage.LastUsed.After(after) {
		t.Errorf("LastUsed timestamp %v is not between %v and %v", 
			usage.LastUsed, before, after)
	}
}

func TestHistoryFileLocation(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)
	
	h := New()
	h.RecordUsage("test", nil)
	
	if err := h.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	
	expectedPath := filepath.Join(tmpDir, ".config", "basin", "history.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("History file not created at expected location: %s", expectedPath)
	}
}
