package gui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/mdelapenya/biomelab/internal/git"
	"github.com/mdelapenya/biomelab/internal/ops"
)

// applyRealTheme swaps the bare test theme for the real default theme so
// Rebuild can resolve fonts (the test theme has no bold-monospace face,
// which is what build()/monoText use).
func applyRealTheme(t *testing.T) {
	t.Helper()
	test.ApplyTheme(t, theme.DefaultTheme())
}

// TestApplyRefresh_DropsStaleSnapshot is the regression guard for the
// "deleted card briefly reappears" race: a long-running refresh that
// snapshotted before a delete (lower Generation) must NOT overwrite the
// dashboard state with its pre-delete worktree list.
func TestApplyRefresh_DropsStaleSnapshot(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	applyRealTheme(t)

	state := &RepoState{}
	d := NewDashboard(state)

	// Apply a "post-mutation" snapshot at gen=5 with only the main wt.
	post := ops.RefreshResult{
		Worktrees:  []git.Worktree{{Path: "/repo", Branch: "main", IsMain: true}},
		Generation: 5,
	}
	if !d.ApplyRefresh(post) {
		t.Fatal("ApplyRefresh(post) returned false; expected the fresh result to apply")
	}
	if state.LastAppliedGen != 5 {
		t.Errorf("LastAppliedGen = %d, want 5", state.LastAppliedGen)
	}
	if got := len(state.Worktrees); got != 1 {
		t.Fatalf("Worktrees after post-apply: %d, want 1", got)
	}

	// Now apply the stale snapshot (gen=3) carrying the doomed branch.
	stale := ops.RefreshResult{
		Worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main", IsMain: true},
			{Name: "doomed", Path: "/wt/doomed", Branch: "doomed"},
		},
		Generation: 3,
	}
	if d.ApplyRefresh(stale) {
		t.Fatal("ApplyRefresh(stale) returned true; expected pre-mutation snapshot to be dropped")
	}
	if got := len(state.Worktrees); got != 1 {
		t.Errorf("Worktrees after stale drop: %d, want 1 (stale snapshot must not overwrite)", got)
	}
	if state.LastAppliedGen != 5 {
		t.Errorf("LastAppliedGen moved after stale drop: %d, want 5", state.LastAppliedGen)
	}
}

// TestApplyRefresh_SameGenerationApplies confirms that snapshots taken at
// the same generation as the last applied one are still applied — they
// represent a fresh observation of the same logical state and may bring
// updated agent/IDE/PR detail.
func TestApplyRefresh_SameGenerationApplies(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	applyRealTheme(t)

	state := &RepoState{}
	d := NewDashboard(state)

	r1 := ops.RefreshResult{
		Worktrees:  []git.Worktree{{Path: "/repo", Branch: "main", IsMain: true}},
		Generation: 0,
	}
	if !d.ApplyRefresh(r1) {
		t.Fatal("first ApplyRefresh dropped")
	}
	r2 := ops.RefreshResult{
		Worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main", IsMain: true},
			{Name: "wt1", Path: "/wt/wt1", Branch: "wt1"},
		},
		Generation: 0,
	}
	if !d.ApplyRefresh(r2) {
		t.Fatal("second ApplyRefresh at same gen dropped; expected apply")
	}
	if got := len(state.Worktrees); got != 2 {
		t.Errorf("Worktrees: %d, want 2", got)
	}
}

// TestApplyRefresh_ErrorAlwaysPasses ensures errors surface even when the
// snapshot's generation would otherwise mark it stale, so users still see
// failure messages from in-flight refreshes that completed late.
func TestApplyRefresh_ErrorAlwaysPasses(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	applyRealTheme(t)

	state := &RepoState{LastAppliedGen: 10}
	d := NewDashboard(state)

	stale := ops.RefreshResult{
		Err:        errFakeRefresh,
		Generation: 2,
	}
	if !d.ApplyRefresh(stale) {
		t.Fatal("ApplyRefresh dropped a stale error result; errors should always surface")
	}
	if state.StatusMessage == "" || !state.StatusIsError {
		t.Errorf("StatusMessage=%q StatusIsError=%v; want non-empty error",
			state.StatusMessage, state.StatusIsError)
	}
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

var errFakeRefresh = fakeErr("simulated refresh failure")
