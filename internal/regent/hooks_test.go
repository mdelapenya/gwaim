package regent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// readSettings loads .claude/settings.json from wt and returns its decoded
// map. Used by every test below — both as input setup and as the assertion
// target after EnsureClaudeHooks runs.
func readSettings(t *testing.T, wt string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(wt, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal settings.json: %v", err)
	}
	return got
}

func hookCommandsFor(t *testing.T, settings map[string]any, event string) []string {
	t.Helper()
	hooks, _ := settings["hooks"].(map[string]any)
	groups := normalizeHookGroups(hooks[event])
	out := []string{}
	for _, group := range groups {
		gm, ok := group.(map[string]any)
		if !ok {
			continue
		}
		entries, _ := normalizeHookEntries(gm["hooks"])
		for _, e := range entries {
			em, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := em["command"].(string); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}

func TestEnsureClaudeHooks_FreshInstall(t *testing.T) {
	wt := t.TempDir()

	if err := EnsureClaudeHooks(wt); err != nil {
		t.Fatalf("EnsureClaudeHooks() = %v", err)
	}

	settings := readSettings(t, wt)
	wantEvents := map[string]string{
		"UserPromptSubmit": claudeUserHook,
		"Stop":             claudeAssistantHook,
		"PostToolBatch":    claudeToolBatchHook,
	}
	for event, want := range wantEvents {
		cmds := hookCommandsFor(t, settings, event)
		if len(cmds) != 1 || cmds[0] != want {
			t.Errorf("%s commands = %v, want [%q]", event, cmds, want)
		}
	}
}

func TestEnsureClaudeHooks_IsIdempotent(t *testing.T) {
	wt := t.TempDir()

	for range 3 {
		if err := EnsureClaudeHooks(wt); err != nil {
			t.Fatalf("EnsureClaudeHooks() = %v", err)
		}
	}
	settings := readSettings(t, wt)
	cmds := hookCommandsFor(t, settings, "UserPromptSubmit")
	if len(cmds) != 1 {
		t.Errorf("UserPromptSubmit has %d entries after 3 calls, want 1: %v", len(cmds), cmds)
	}
}

func TestEnsureClaudeHooks_PreservesUserHooks(t *testing.T) {
	wt := t.TempDir()
	claudeDir := filepath.Join(wt, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo not-rgt"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureClaudeHooks(wt); err != nil {
		t.Fatalf("EnsureClaudeHooks() = %v", err)
	}

	settings := readSettings(t, wt)
	cmds := hookCommandsFor(t, settings, "UserPromptSubmit")
	if len(cmds) != 2 {
		t.Fatalf("UserPromptSubmit has %d entries, want 2 (user + rgt): %v", len(cmds), cmds)
	}
	if cmds[0] != "echo not-rgt" {
		t.Errorf("UserPromptSubmit[0] = %q, want %q (user hook preserved first)", cmds[0], "echo not-rgt")
	}
	if cmds[1] != claudeUserHook {
		t.Errorf("UserPromptSubmit[1] = %q, want %q (rgt appended)", cmds[1], claudeUserHook)
	}
}

func TestEnsureClaudeHooks_BacksUpInvalidJSON(t *testing.T) {
	wt := t.TempDir()
	claudeDir := filepath.Join(wt, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureClaudeHooks(wt); err != nil {
		t.Fatalf("EnsureClaudeHooks() = %v", err)
	}

	if _, err := os.Stat(settingsPath + ".backup"); err != nil {
		t.Errorf("backup file not created: %v", err)
	}
	if cmds := hookCommandsFor(t, readSettings(t, wt), "UserPromptSubmit"); len(cmds) != 1 {
		t.Errorf("UserPromptSubmit entries after recovery = %d, want 1", len(cmds))
	}
}

func TestEnsureClaudeHooks_RemovesLegacyPostToolUse(t *testing.T) {
	// Older rgt versions wrote PostToolUse; current installer writes
	// PostToolBatch. EnsureClaudeHooks should clean up the legacy entry.
	wt := t.TempDir()
	claudeDir := filepath.Join(wt, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{"type": "command", "command": "rgt some-old-hook"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureClaudeHooks(wt); err != nil {
		t.Fatalf("EnsureClaudeHooks() = %v", err)
	}

	settings := readSettings(t, wt)
	hooks, _ := settings["hooks"].(map[string]any)
	if _, exists := hooks["PostToolUse"]; exists {
		t.Errorf("PostToolUse still present after cleanup; settings: %+v", hooks)
	}
}

func TestIsRgtCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"rgt message-hook user", true},
		{"regent something", true},
		{"NO_COLOR=1 rgt message-hook user", true},
		{"go run ./cmd/rgt message-hook user", true},
		{"echo hello", false},
		{"", false},
		{"  ", false},
		{"/usr/local/bin/rgt message-hook user", true},
	}
	for _, tt := range cases {
		if got := isRgtCommand(tt.cmd); got != tt.want {
			t.Errorf("isRgtCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}
