package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/mdelapenya/biomelab/internal/git"
	"github.com/mdelapenya/biomelab/internal/provider"
)

// TestKanbanStages_AlwaysFiveStages verifies that KanbanStages always returns
// exactly five stage buckets — even when some (or all) are empty — so that
// buildKanbanView can always produce a complete five-column board.
func TestKanbanStages_AlwaysFiveStages(t *testing.T) {
	cases := []struct {
		name      string
		worktrees []git.Worktree
		prs       provider.PRResult
	}{
		{
			name: "no linked worktrees",
			worktrees: []git.Worktree{
				{Path: "/repo", Branch: "main", IsMain: true},
			},
		},
		{
			name: "all stages populated",
			worktrees: []git.Worktree{
				{Path: "/repo", Branch: "main", IsMain: true},
				{Name: "closed-unmerged", Path: "/wt/closed-unmerged", Branch: "closed-unmerged"},
				{Name: "created", Path: "/wt/created", Branch: "created"},
				{Name: "sent", Path: "/wt/sent", Branch: "sent"},
				{Name: "review", Path: "/wt/review", Branch: "review"},
				{Name: "merged", Path: "/wt/merged", Branch: "merged"},
			},
			prs: provider.PRResult{
				"closed-unmerged": {State: "closed"},
				"sent":            {State: "open"},
				"review":          {State: "open", ReviewStatus: "approved"},
				"merged":          {State: "merged"},
			},
		},
		{
			name: "only stage 1 (no PRs)",
			worktrees: []git.Worktree{
				{Path: "/repo", Branch: "main", IsMain: true},
				{Name: "wt1", Path: "/wt/wt1", Branch: "wt1"},
				{Name: "wt2", Path: "/wt/wt2", Branch: "wt2"},
			},
		},
		{
			name: "stages 1 and 2 only — stages 0, 3, 4 are empty",
			worktrees: []git.Worktree{
				{Path: "/repo", Branch: "main", IsMain: true},
				{Name: "no-pr", Path: "/wt/no-pr", Branch: "no-pr"},
				{Name: "open-pr", Path: "/wt/open-pr", Branch: "open-pr"},
			},
			prs: provider.PRResult{
				"open-pr": {State: "open"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Dashboard{
				state: &RepoState{
					Worktrees: tc.worktrees,
					PRs:       tc.prs,
				},
			}
			stages := d.KanbanStages()
			// Each linked worktree must appear in exactly one stage bucket.
			linked := len(d.state.LinkedWorktrees())
			total := 0
			for _, bucket := range stages {
				total += len(bucket)
			}
			if total != linked {
				t.Errorf("stages contain %d worktree indices, want %d (one per linked worktree)", total, linked)
			}
		})
	}
}

// TestBuildKanbanView_AlwaysFiveColumns verifies that buildKanbanView always
// produces a grid container with exactly five children — one per stage column —
// even when some stages are empty. Regression guard for the "lost-columns" bug
// where an empty PR-Merged column would collapse and disappear from the board.
func TestBuildKanbanView_AlwaysFiveColumns(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	cases := []struct {
		name      string
		worktrees []git.Worktree
		prs       provider.PRResult
	}{
		{
			name: "stage 4 empty — only stages 1+2 occupied",
			worktrees: []git.Worktree{
				{Path: "/repo", Branch: "main", IsMain: true},
				{Name: "no-pr", Path: "/wt/no-pr", Branch: "no-pr"},
				{Name: "open-pr", Path: "/wt/open-pr", Branch: "open-pr"},
			},
			prs: provider.PRResult{
				"open-pr": {State: "open"},
			},
		},
		{
			name: "all stages empty",
			worktrees: []git.Worktree{
				{Path: "/repo", Branch: "main", IsMain: true},
				{Name: "wt1", Path: "/wt/wt1", Branch: "wt1"},
			},
		},
		{
			name: "all five stages occupied",
			worktrees: []git.Worktree{
				{Path: "/repo", Branch: "main", IsMain: true},
				{Name: "closed-unmerged", Path: "/wt/closed-unmerged", Branch: "closed-unmerged"},
				{Name: "created", Path: "/wt/created", Branch: "created"},
				{Name: "sent", Path: "/wt/sent", Branch: "sent"},
				{Name: "review", Path: "/wt/review", Branch: "review"},
				{Name: "merged", Path: "/wt/merged", Branch: "merged"},
			},
			prs: provider.PRResult{
				"closed-unmerged": {State: "closed"},
				"sent":            {State: "open"},
				"review":          {State: "open", ReviewStatus: "approved"},
				"merged":          {State: "merged"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Dashboard{
				state: &RepoState{
					Worktrees: tc.worktrees,
					PRs:       tc.prs,
					ViewMode:  ViewKanban,
				},
			}

			result := d.buildKanbanView()

			grid, ok := result.(*fyne.Container)
			if !ok {
				t.Fatalf("buildKanbanView did not return *fyne.Container, got %T", result)
			}
			if got := len(grid.Objects); got != 5 {
				t.Errorf("kanban grid has %d columns, want 5", got)
			}
			for i, obj := range grid.Objects {
				if obj == nil {
					t.Errorf("column %d is nil", i)
				} else if !obj.Visible() {
					t.Errorf("column %d is not visible", i)
				}
			}
		})
	}
}
