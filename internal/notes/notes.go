// Package notes manages per-worktree scratchpad notes stored as markdown.
//
// A note for a worktree lives at <worktreeDir>/.biomelab/note.md and is kept
// invisible to git via the per-worktree info/exclude file, so it never shows
// up in "git status" and cannot leak into a commit by accident. Notes are
// inside the worktree directory on purpose: the sandbox mounts the worktree,
// so anything stored here is reachable by an agent running in the microVM.
package notes

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	noteDir      = ".biomelab"
	noteFile     = "note.md"
	prTitleFile  = "pr-title.md"
	excludeLine  = "/.biomelab/"
	dirPerm      = 0o755
	filePerm     = 0o644
)

// Path returns the absolute path to the note file for a worktree directory.
func Path(worktreeDir string) string {
	return filepath.Join(worktreeDir, noteDir, noteFile)
}

// TitlePath returns the absolute path to the PR title file for a worktree.
// External tools (e.g. the pr-scribe skill) write a single-line Conventional
// Commits title here; biomelab uses its content as `--title` when the user
// opts to include task notes during the PR send flow.
func TitlePath(worktreeDir string) string {
	return filepath.Join(worktreeDir, noteDir, prTitleFile)
}

// ReadTitle returns the trimmed first non-empty line of the PR title file.
// ok is false when the file does not exist or contains only whitespace; in
// either case title is "" and err is nil. Only the first line is honored
// because Git/CLI title arguments are single-line by design.
func ReadTitle(worktreeDir string) (title string, ok bool, err error) {
	data, err := os.ReadFile(TitlePath(worktreeDir))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", false, nil
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false, nil
	}
	return text, true, nil
}

// Exists reports whether a note file exists for the given worktree.
func Exists(worktreeDir string) bool {
	_, err := os.Stat(Path(worktreeDir))
	return err == nil
}

// Read returns the note content. The bool is false when no note file exists,
// in which case content is empty and err is nil.
func Read(worktreeDir string) (string, bool, error) {
	data, err := os.ReadFile(Path(worktreeDir))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}

// Write saves the note and ensures the worktree's info/exclude file ignores
// the .biomelab/ directory. Trailing whitespace is stripped and a single
// trailing newline is appended so accidental Enter-mashing at the end of
// the editor doesn't leave blank lines on disk. An all-whitespace content
// is treated as a delete.
func Write(worktreeDir, content string) error {
	content = strings.TrimRight(content, " \t\r\n")
	if content == "" {
		return Delete(worktreeDir)
	}
	content += "\n"
	dir := filepath.Join(worktreeDir, noteDir)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create note dir: %w", err)
	}
	if err := os.WriteFile(Path(worktreeDir), []byte(content), filePerm); err != nil {
		return fmt.Errorf("write note: %w", err)
	}
	if err := ensureExcluded(worktreeDir); err != nil {
		return fmt.Errorf("ensure excluded: %w", err)
	}
	return nil
}

// Delete removes the note file. Returns nil if the note doesn't exist.
func Delete(worktreeDir string) error {
	err := os.Remove(Path(worktreeDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// excludeFilePath resolves the path to the info/exclude file git actually
// consults for this worktree. Git reads info/exclude from the COMMON gitdir
// for both the main worktree and any linked worktrees — the per-worktree
// .git/worktrees/<name>/info/exclude is created by `git worktree add` but
// is not honored by `git status` (verified empirically). So:
//
//   - main worktree: .git is a directory → exclude at .git/info/exclude
//   - linked worktree: .git is a file with "gitdir: <wt-gitdir>" → read
//     <wt-gitdir>/commondir to find the common gitdir, exclude at
//     <common-gitdir>/info/exclude.
//
// As a side effect, all worktrees of a repo share the same exclude line.
// That's fine: the pattern "/.biomelab/" is anchored to the worktree root
// at match time, so each worktree's own .biomelab/ stays hidden.
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

// ensureExcluded appends excludeLine to the worktree's info/exclude if it
// isn't already there. Idempotent.
func ensureExcluded(worktreeDir string) error {
	excludePath, err := excludeFilePath(worktreeDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), dirPerm); err != nil {
		return fmt.Errorf("create info dir: %w", err)
	}
	existing, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read exclude: %w", err)
	}
	if hasExcludeLine(existing, excludeLine) {
		return nil
	}
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("open exclude: %w", err)
	}
	var buf strings.Builder
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString(excludeLine)
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

func hasExcludeLine(content []byte, line string) bool {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == line {
			return true
		}
	}
	return false
}
