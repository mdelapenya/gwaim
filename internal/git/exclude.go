package git

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// excludeDirPerm and excludeFilePerm are the standard git info/exclude
// file permissions (mirrors what `git worktree add` produces).
const (
	excludeDirPerm  = 0o755
	excludeFilePerm = 0o644
)

// EnsureExcluded appends line to the worktree's info/exclude file if it
// isn't already present. Idempotent. Used by sidecar packages (notes,
// regent) to hide per-worktree directories from `git status`.
//
// Git reads info/exclude from the common gitdir for both the main worktree
// and any linked worktrees — the per-worktree
// .git/worktrees/<name>/info/exclude is created by `git worktree add` but
// is not honored by `git status` (verified empirically). This function
// resolves the right file in both cases:
//
//   - main worktree: .git is a directory → <wt>/.git/info/exclude
//   - linked worktree: .git is a file with "gitdir: <wt-gitdir>" → read
//     <wt-gitdir>/commondir to find the common gitdir, then
//     <common-gitdir>/info/exclude.
//
// As a side effect, all worktrees of a repo share the same exclude file.
// Patterns are anchored to the worktree root at match time (e.g.
// "/.regent/"), so each worktree's own sidecar dir stays hidden.
func EnsureExcluded(worktreeDir, line string) error {
	excludePath, err := excludeFilePath(worktreeDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), excludeDirPerm); err != nil {
		return fmt.Errorf("create info dir: %w", err)
	}
	existing, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read exclude: %w", err)
	}
	if hasExcludeLine(existing, line) {
		return nil
	}
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, excludeFilePerm)
	if err != nil {
		return fmt.Errorf("open exclude: %w", err)
	}
	var buf strings.Builder
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString(line)
	buf.WriteString("\n")
	if _, werr := f.WriteString(buf.String()); werr != nil {
		_ = f.Close()
		return fmt.Errorf("write exclude: %w", werr)
	}
	if cerr := f.Close(); cerr != nil {
		return fmt.Errorf("close exclude: %w", cerr)
	}
	return nil
}

func excludeFilePath(worktreeDir string) (string, error) {
	gitPath := filepath.Join(worktreeDir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", fmt.Errorf("stat .git: %w", err)
	}
	if info.IsDir() {
		return filepath.Join(gitPath, "info", "exclude"), nil
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", fmt.Errorf("read .git: %w", err)
	}
	const prefix = "gitdir:"
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf(".git file missing %q prefix", prefix)
	}
	wtGitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))

	commonDir := wtGitDir
	if raw, rerr := os.ReadFile(filepath.Join(wtGitDir, "commondir")); rerr == nil {
		cd := strings.TrimSpace(string(raw))
		if !filepath.IsAbs(cd) {
			cd = filepath.Join(wtGitDir, cd)
		}
		commonDir = filepath.Clean(cd)
	}
	return filepath.Join(commonDir, "info", "exclude"), nil
}

func hasExcludeLine(content []byte, line string) bool {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == line {
			return true
		}
	}
	return false
}
