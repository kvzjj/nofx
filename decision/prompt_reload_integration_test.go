package decision

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPromptReloadEndToEnd verifies the reload flow used by the runtime:
// file modification on disk -> ReloadPromptTemplates() -> GetPromptTemplate()
// serves the updated content.
//
// Prompt content itself is assembled by StrategyEngine.BuildSystemPrompt from
// store.StrategyConfig, so these tests intentionally stay at the template
// manager level.
func TestPromptReloadEndToEnd(t *testing.T) {
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	tempDir := t.TempDir()
	promptsDir = tempDir

	// Step 1: create the initial prompt file.
	initialContent := "# initial strategy\nconservative AI"
	if err := os.WriteFile(filepath.Join(tempDir, "test_strategy.txt"), []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	// Step 2: first load (simulates process start).
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("initial load failed: %v", err)
	}

	// Step 3: verify the initial content is served.
	template, err := GetPromptTemplate("test_strategy")
	if err != nil {
		t.Fatalf("failed to get initial template: %v", err)
	}
	if template.Content != initialContent {
		t.Errorf("initial content mismatch\nwant: %s\ngot:  %s", initialContent, template.Content)
	}

	// Step 4: simulate an operator editing the file on disk.
	updatedContent := "# updated strategy\naggressive AI"
	if err := os.WriteFile(filepath.Join(tempDir, "test_strategy.txt"), []byte(updatedContent), 0644); err != nil {
		t.Fatalf("failed to update file: %v", err)
	}

	// Step 5: reload as the trader start path does.
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	// Step 6: the updated content must be served without a process restart.
	reloaded, err := GetPromptTemplate("test_strategy")
	if err != nil {
		t.Fatalf("failed to get reloaded template: %v", err)
	}
	if reloaded.Content != updatedContent {
		t.Errorf("reloaded content mismatch\nwant: %s\ngot:  %s", updatedContent, reloaded.Content)
	}
}

// TestConcurrentPromptReload exercises readers racing with reloads; the
// manager must serve consistent snapshots without data corruption.
func TestConcurrentPromptReload(t *testing.T) {
	originalDir := promptsDir
	defer func() {
		promptsDir = originalDir
		globalPromptManager.ReloadTemplates(originalDir)
	}()

	tempDir := t.TempDir()
	promptsDir = tempDir

	if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("stable content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := ReloadPromptTemplates(); err != nil {
		t.Fatalf("initial load failed: %v", err)
	}

	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _ = GetPromptTemplate("test")
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = ReloadPromptTemplates()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 13; i++ {
		<-done
	}

	template, err := GetPromptTemplate("test")
	if err != nil {
		t.Errorf("failed to get template after concurrent access: %v", err)
	} else if template.Content != "stable content" {
		t.Errorf("template content corrupted by concurrent reload: %s", template.Content)
	}
}
