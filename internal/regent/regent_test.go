package regent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInspect_NoRegentDir(t *testing.T) {
	dir := t.TempDir()
	got := Inspect(dir)
	if got.Enabled {
		t.Fatalf("Enabled = true, want false for workspace without .regent/")
	}
	if !got.LastTouch.IsZero() {
		t.Errorf("LastTouch = %v, want zero", got.LastTouch)
	}
	if got.Sessions != 0 {
		t.Errorf("Sessions = %d, want 0", got.Sessions)
	}
}

func TestInspect_EmptyWorkspaceDir(t *testing.T) {
	got := Inspect("")
	if got.Enabled {
		t.Fatalf("Enabled = true for empty workspaceDir, want false")
	}
}

func TestInspect_RegentDirIsFile(t *testing.T) {
	dir := t.TempDir()
	// .regent as a regular file should not count as enabled.
	if err := os.WriteFile(filepath.Join(dir, ".regent"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Inspect(dir)
	if got.Enabled {
		t.Errorf("Enabled = true when .regent is a file, want false")
	}
}

func TestInspect_EnabledMinimal(t *testing.T) {
	dir := t.TempDir()
	regentDir := filepath.Join(dir, ".regent")
	if err := os.Mkdir(regentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	got := Inspect(dir)
	if !got.Enabled {
		t.Errorf("Enabled = false, want true for empty .regent/")
	}
	if !got.LastTouch.IsZero() {
		t.Errorf("LastTouch = %v, want zero (no index.db, no objects)", got.LastTouch)
	}
	if got.Sessions != 0 {
		t.Errorf("Sessions = %d, want 0", got.Sessions)
	}
}

func TestInspect_IndexDBMtime(t *testing.T) {
	dir := t.TempDir()
	regentDir := filepath.Join(dir, ".regent")
	if err := os.Mkdir(regentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(regentDir, "index.db")
	if err := os.WriteFile(dbPath, []byte("fake sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(dbPath, want, want); err != nil {
		t.Fatal(err)
	}

	got := Inspect(dir)
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if !got.LastTouch.Equal(want) {
		t.Errorf("LastTouch = %v, want %v", got.LastTouch, want)
	}
}

func TestInspect_ObjectsFallback(t *testing.T) {
	dir := t.TempDir()
	objectsDir := filepath.Join(dir, ".regent", "objects", "ab")
	if err := os.MkdirAll(objectsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	older := filepath.Join(objectsDir, "blob1")
	newer := filepath.Join(objectsDir, "blob2")
	if err := os.WriteFile(older, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	tOld := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	tNew := time.Now().Add(-30 * time.Minute).Truncate(time.Second)
	if err := os.Chtimes(older, tOld, tOld); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, tNew, tNew); err != nil {
		t.Fatal(err)
	}

	got := Inspect(dir)
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if !got.LastTouch.Equal(tNew) {
		t.Errorf("LastTouch = %v, want newest blob mtime %v", got.LastTouch, tNew)
	}
}

func TestInspect_IndexDBWinsOverObjects(t *testing.T) {
	// When index.db exists, its mtime is the canonical signal even if
	// objects/ contains newer blob files. The fallback is only for the
	// brief window between .regent/ creation and the first SQLite write.
	dir := t.TempDir()
	regentDir := filepath.Join(dir, ".regent")
	objectsDir := filepath.Join(regentDir, "objects", "ab")
	if err := os.MkdirAll(objectsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(regentDir, "index.db")
	if err := os.WriteFile(dbPath, []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}
	blobPath := filepath.Join(objectsDir, "blob")
	if err := os.WriteFile(blobPath, []byte("blob"), 0o644); err != nil {
		t.Fatal(err)
	}
	dbT := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	blobT := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	if err := os.Chtimes(dbPath, dbT, dbT); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(blobPath, blobT, blobT); err != nil {
		t.Fatal(err)
	}

	got := Inspect(dir)
	if !got.LastTouch.Equal(dbT) {
		t.Errorf("LastTouch = %v, want index.db mtime %v", got.LastTouch, dbT)
	}
}

func TestInspect_SessionsCount(t *testing.T) {
	dir := t.TempDir()
	sessionsDir := filepath.Join(dir, ".regent", "refs", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"session-1", "session-2", "session-3"} {
		if err := os.WriteFile(filepath.Join(sessionsDir, name), []byte("ref"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := Inspect(dir)
	if got.Sessions != 3 {
		t.Errorf("Sessions = %d, want 3", got.Sessions)
	}
}
