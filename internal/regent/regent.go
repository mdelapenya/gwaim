// Package regent inspects a worktree's .regent/ directory to surface re_gent
// (https://github.com/regent-vcs/re_gent) audit-log activity in biomelab.
//
// re_gent is a content-addressed audit log of AI agent activity, captured via
// Claude Code hooks. It writes everything under <workspace>/.regent/. Because
// the directory is on the host filesystem in regular mode and bind-mounted
// from the container in sandbox mode, biomelab can read the same data in both
// modes without any container introspection.
//
// This package intentionally does not shell out to the rgt CLI or open the
// SQLite index. The schema is unstable at v0.1.2 and a small filesystem-only
// read is enough for a card-level activity hint. Heavier integrations (full
// session timeline, blame-in-diff) can layer on later.
package regent

import (
	"os"
	"path/filepath"
	"time"
)

// Status describes the regent state of a single worktree.
// Zero value means regent is not enabled for that worktree.
type Status struct {
	Enabled   bool      // .regent/ exists under the workspace
	LastTouch time.Time // mtime of the most recent activity signal (index.db, fallback objects/)
	Sessions  int       // count of entries under refs/sessions/
}

// Inspect reads <workspaceDir>/.regent/ and returns a Status. Missing or
// unreadable directories yield a zero-value Status (Enabled=false) with no
// error — regent is opt-in and most worktrees won't have it.
func Inspect(workspaceDir string) Status {
	if workspaceDir == "" {
		return Status{}
	}
	regentDir := filepath.Join(workspaceDir, ".regent")
	info, err := os.Stat(regentDir)
	if err != nil || !info.IsDir() {
		return Status{}
	}

	s := Status{Enabled: true}

	if dbInfo, err := os.Stat(filepath.Join(regentDir, "index.db")); err == nil {
		s.LastTouch = dbInfo.ModTime()
	} else if mt, ok := newestObjectMtime(filepath.Join(regentDir, "objects")); ok {
		s.LastTouch = mt
	}

	if entries, err := os.ReadDir(filepath.Join(regentDir, "refs", "sessions")); err == nil {
		s.Sessions = len(entries)
	}

	return s
}

// newestObjectMtime walks one level into objects/<shard>/ and returns the
// most recent mtime among blob files. Used as a fallback when index.db does
// not exist yet (very fresh .regent/ before the first SQLite write).
func newestObjectMtime(objectsDir string) (time.Time, bool) {
	shards, err := os.ReadDir(objectsDir)
	if err != nil {
		return time.Time{}, false
	}
	var newest time.Time
	for _, shard := range shards {
		if !shard.IsDir() {
			continue
		}
		blobs, err := os.ReadDir(filepath.Join(objectsDir, shard.Name()))
		if err != nil {
			continue
		}
		for _, b := range blobs {
			info, err := b.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
	}
	return newest, !newest.IsZero()
}
