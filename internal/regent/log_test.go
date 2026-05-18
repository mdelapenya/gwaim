package regent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLog_EmptyPath(t *testing.T) {
	out, err := Log("", 10)
	if err != nil {
		t.Errorf("Log(\"\", 10) returned %v, want nil", err)
	}
	if out != "" {
		t.Errorf("Log(\"\", 10) returned %q, want empty", out)
	}
}

func TestLog_NoRegentDir(t *testing.T) {
	// Worktree without .regent/ → empty output, no error.
	dir := t.TempDir()
	out, err := Log(dir, 10)
	if err != nil {
		t.Errorf("Log() = err %v, want nil", err)
	}
	if out != "" {
		t.Errorf("Log() = %q, want empty", out)
	}
}

func TestLog_RgtMissingFromPath(t *testing.T) {
	// .regent/ exists but rgt is not on PATH → error.
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".regent"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Log(dir, 10)
	if err == nil {
		t.Error("Log() returned nil error when rgt is missing")
	}
}

func TestStripANSI(t *testing.T) {
	in := "\x1b[38;5;141mStep abc\x1b[0m | 2 min ago"
	want := "Step abc | 2 min ago"
	if got := stripANSI(in); got != want {
		t.Errorf("stripANSI() = %q, want %q", got, want)
	}
}

func TestStripANSI_NoEscapes(t *testing.T) {
	in := "plain text"
	if got := stripANSI(in); got != in {
		t.Errorf("stripANSI(%q) = %q, want unchanged", in, got)
	}
}

func TestStripANSI_OSC(t *testing.T) {
	in := "before\x1b]0;title text\x07after"
	want := "beforeafter"
	if got := stripANSI(in); !strings.Contains(got, "before") || !strings.Contains(got, "after") || got != want {
		t.Errorf("stripANSI() = %q, want %q", got, want)
	}
}
