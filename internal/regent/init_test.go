package regent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeWorktree returns a tempdir set up to look like a main worktree:
// a real .git/ directory so notes.EnsureExcluded can write into
// .git/info/exclude without going through go-git init.
func fakeWorktree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readExclude(t *testing.T, wtDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(wtDir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	return string(data)
}

func TestEnsureInit_EmptyPath(t *testing.T) {
	if err := EnsureInit(""); err != nil {
		t.Errorf("EnsureInit(\"\") returned %v, want nil", err)
	}
}

func TestEnsureInit_RegentDirExists_WritesExclude(t *testing.T) {
	// When .regent/ already exists, EnsureInit skips the rgt invocation
	// but still ensures the git exclude line is present — the dir may
	// pre-date this code path or have been created by another tool.
	dir := fakeWorktree(t)
	if err := os.Mkdir(filepath.Join(dir, ".regent"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := EnsureInit(dir); err != nil {
		t.Fatalf("EnsureInit() = %v, want nil", err)
	}

	contents := readExclude(t, dir)
	if !strings.Contains(contents, "/.regent/") {
		t.Errorf("info/exclude missing /.regent/ entry; got:\n%s", contents)
	}
}

func TestEnsureInit_ExcludeIsIdempotent(t *testing.T) {
	// Calling EnsureInit twice must not duplicate the exclude line.
	dir := fakeWorktree(t)
	if err := os.Mkdir(filepath.Join(dir, ".regent"), 0o755); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if err := EnsureInit(dir); err != nil {
			t.Fatalf("EnsureInit() = %v", err)
		}
	}
	contents := readExclude(t, dir)
	if got := strings.Count(contents, "/.regent/"); got != 1 {
		t.Errorf("/.regent/ appears %d times in info/exclude, want 1; contents:\n%s", got, contents)
	}
}

func TestEnsureInit_RegentMissingFromPath(t *testing.T) {
	// Without rgt in PATH, EnsureInit must no-op. We force this by
	// pointing PATH at a tempdir with no executables — a deliberate
	// reset rather than depending on what the host machine has installed.
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	if err := EnsureInit(dir); err != nil {
		t.Errorf("EnsureInit() = %v, want nil (no-op when rgt missing)", err)
	}
	// Confirm we did NOT create .regent/ as a side-effect.
	if _, err := os.Stat(filepath.Join(dir, ".regent")); err == nil {
		t.Error(".regent/ was created even though rgt is not in PATH")
	}
}

func TestEnsureInit_RegentDirIsFile(t *testing.T) {
	// A .regent file (not a dir) shouldn't be mistaken for an existing
	// init. We need to fall through to the rgt invocation (or, in this
	// test, the rgt-missing branch).
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".regent"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// rgt not in PATH ⇒ no-op, returning nil; the existing file is not
	// treated as a valid init.
	if err := EnsureInit(dir); err != nil {
		t.Errorf("EnsureInit() = %v, want nil", err)
	}
}

func TestMigrateAll_AcceptsEmpty(t *testing.T) {
	// MigrateAll on an empty slice must not panic and must not call rgt.
	t.Setenv("PATH", t.TempDir())
	MigrateAll(nil)
	MigrateAll([]string{})
}

func TestMigrateAll_IgnoresErrors(t *testing.T) {
	// MigrateAll continues past failing entries. With rgt missing,
	// EnsureInit returns nil for each path; this exercises the loop.
	t.Setenv("PATH", t.TempDir())
	paths := []string{t.TempDir(), t.TempDir(), ""}
	MigrateAll(paths)
}
