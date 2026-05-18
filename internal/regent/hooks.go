package regent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Claude Code hook commands written into .claude/settings.json. The names
// mirror what rgt's own installer uses; ported here so biomelab can wire
// the hooks without an interactive TTY (the upstream `rgt init` flow only
// installs hooks after a `huh`-based TTY prompt that fails under a GUI
// launch).
//
// Source: github.com/regent-vcs/re_gent internal/cli/init.go (Apache-2.0).
const (
	claudeUserHook      = "rgt message-hook user"
	claudeAssistantHook = "rgt message-hook assistant"
	claudeToolBatchHook = "rgt tool-batch-hook"
)

// EnsureClaudeHooks writes Claude Code hook entries into
// <wtPath>/.claude/settings.json. Idempotent: existing rgt-related entries
// are replaced (not duplicated), non-rgt entries are preserved. Returns
// nil when wtPath is empty. Backs up unparseable settings files to
// settings.json.backup before overwriting.
//
// The JSON shape matches what `rgt init` writes under the interactive
// installer:
//
//	{
//	  "hooks": {
//	    "UserPromptSubmit": [{"matcher": "", "hooks": [{"type": "command", "command": "rgt message-hook user"}]}],
//	    "Stop": [{...}],
//	    "PostToolBatch": [{...}]
//	  }
//	}
func EnsureClaudeHooks(wtPath string) error {
	if wtPath == "" {
		return nil
	}
	claudeDir := filepath.Join(wtPath, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude/: %w", err)
	}

	settings := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if jerr := json.Unmarshal(data, &settings); jerr != nil {
			backup := settingsPath + ".backup"
			if rerr := os.Rename(settingsPath, backup); rerr != nil {
				return fmt.Errorf("backup invalid settings.json: %w", rerr)
			}
			settings = map[string]any{}
		}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}

	mergeClaudeHook(hooks, "UserPromptSubmit", claudeUserHook)
	mergeClaudeHook(hooks, "Stop", claudeAssistantHook)
	mergeClaudeHook(hooks, "PostToolBatch", claudeToolBatchHook)
	removeClaudeHook(hooks, "PostToolUse") // legacy cleanup

	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings.json: %w", err)
	}
	if err := os.WriteFile(settingsPath, append(output, '\n'), 0o644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}
	return nil
}

// mergeClaudeHook adds/replaces the rgt entry for the given event while
// preserving any other (non-rgt) hook groups the user has configured.
func mergeClaudeHook(hooks map[string]any, event, command string) {
	groups := filterRgtHooks(normalizeHookGroups(hooks[event]))
	hooks[event] = append(groups, hookGroup(command))
}

// removeClaudeHook strips any rgt entry from the given event. Used to
// clean up legacy keys (e.g. an older rgt wrote PostToolUse before
// migrating to PostToolBatch); leaves non-rgt entries intact.
func removeClaudeHook(hooks map[string]any, event string) {
	groups := filterRgtHooks(normalizeHookGroups(hooks[event]))
	if len(groups) == 0 {
		delete(hooks, event)
		return
	}
	hooks[event] = groups
}

// normalizeHookGroups accepts the various JSON-decoded shapes Claude Code
// accepts for an event entry (single object, array of objects, etc.) and
// returns them as a uniform []any.
func normalizeHookGroups(value any) []any {
	switch v := value.(type) {
	case nil:
		return nil
	case []any:
		return v
	case map[string]any:
		return []any{v}
	default:
		return []any{v}
	}
}

// filterRgtHooks returns the input groups with any rgt-related hook
// commands stripped out. Groups that become empty after the strip are
// dropped so an event with only rgt entries vanishes cleanly.
func filterRgtHooks(groups []any) []any {
	filtered := make([]any, 0, len(groups))
	for _, group := range groups {
		gm, ok := group.(map[string]any)
		if !ok {
			filtered = append(filtered, group)
			continue
		}
		entries, hasEntries := normalizeHookEntries(gm["hooks"])
		if !hasEntries {
			filtered = append(filtered, group)
			continue
		}
		next := make([]any, 0, len(entries))
		for _, e := range entries {
			em, ok := e.(map[string]any)
			if !ok {
				next = append(next, e)
				continue
			}
			cmd, _ := em["command"].(string)
			if isRgtCommand(cmd) {
				continue
			}
			next = append(next, em)
		}
		if len(next) == 0 {
			continue
		}
		gm["hooks"] = next
		filtered = append(filtered, gm)
	}
	return filtered
}

func normalizeHookEntries(value any) ([]any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case []any:
		return v, true
	case map[string]any:
		return []any{v}, true
	default:
		return []any{v}, true
	}
}

// hookGroup is the canonical JSON shape for a single Claude Code hook
// entry. Mirrors what rgt's interactive installer writes.
func hookGroup(command string) map[string]any {
	return map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	}
}

// isRgtCommand recognizes any command that invokes rgt — whether by name
// (`rgt …` / `regent …`), via leading env var assignments (`FOO=bar rgt …`),
// or via `go run ./cmd/rgt …` during development. Used to dedupe entries
// across re-runs of EnsureClaudeHooks.
func isRgtCommand(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	for len(fields) > 0 && strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "=") {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return false
	}
	first := strings.TrimPrefix(filepath.Base(fields[0]), "./")
	if first == "rgt" || first == "regent" {
		return true
	}
	return len(fields) >= 3 && fields[0] == "go" && fields[1] == "run" && strings.Contains(fields[2], "cmd/rgt")
}
