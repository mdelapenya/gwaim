package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/mdelapenya/biomelab/internal/config"
	"github.com/mdelapenya/biomelab/internal/sandbox"
)

// RepoGroup represents a registered repository with its modes.
type RepoGroup struct {
	Path                string
	Name                string
	Modes               []config.ModeEntry
	ActiveMode          int
	LinkedWorktreeCount int
}

// RepoPanel is the left panel showing the repository/mode list.
// It does NOT use widget.Tree (which implements Focusable and steals
// keyboard focus). Instead it uses plain tappable containers so the
// canvas-level key handlers always work.
type RepoPanel struct {
	groups      []*RepoGroup
	activeGrp   int
	activeMode  int
	sbxStatuses map[string]sandbox.Status

	content *fyne.Container
	list    *fyne.Container // the scrollable mode list

	// headerRows holds the outer header container for each group so the
	// drag-reorder code can snapshot Y positions without re-walking the
	// list. Index matches group index.
	headerRows []fyne.CanvasObject

	// Drag-reorder state. dragSrc == -1 when no drag is in progress.
	// Header Y positions are snapshotted at drag start (the list isn't
	// rebuilt during the drag, so they stay valid).
	dragSrc          int
	dragDst          int
	dragCumulativeDY float32
	headerYSnap      []float32
	headerHSnap      []float32

	OnModeSelected func(groupIdx, modeIdx int)
	OnReorder      func(fromIdx, toIdx int)
}

// NewRepoPanel creates a repo panel from config entries.
func NewRepoPanel(groups []*RepoGroup, sbxStatuses map[string]sandbox.Status) *RepoPanel {
	rp := &RepoPanel{
		groups:      groups,
		sbxStatuses: sbxStatuses,
		dragSrc:     -1,
		dragDst:     -1,
	}
	rp.build()
	return rp
}

// Content returns the renderable panel.
func (rp *RepoPanel) Content() fyne.CanvasObject {
	return rp.content
}

// SetActive highlights the given group and mode.
func (rp *RepoPanel) SetActive(groupIdx, modeIdx int) {
	rp.activeGrp = groupIdx
	rp.activeMode = modeIdx
	rp.rebuildList()
}

// UpdateStatuses refreshes sandbox status dots.
func (rp *RepoPanel) UpdateStatuses(statuses map[string]sandbox.Status) {
	rp.sbxStatuses = statuses
	rp.rebuildList()
}

func (rp *RepoPanel) build() {
	rp.list = container.NewVBox()
	rp.rebuildList()

	helpLabel := monoText("[a]dd  [n]ew sandbox  [x]rm", colorDimGray, false)
	helpLabel.TextSize = scaledSize(9)

	scroll := container.NewScroll(rp.list)
	bg := canvas.NewRectangle(colorPanelBg)

	inner := container.NewBorder(nil, container.NewPadded(helpLabel), nil, nil, scroll)

	if rp.content == nil {
		rp.content = container.NewStack(bg, inner)
	} else {
		// Replace content in-place so the parent layout keeps the same object.
		rp.content.Objects = []fyne.CanvasObject{bg, inner}
		rp.content.Refresh()
	}
}

// RebuildFull rebuilds the entire panel including help labels (for zoom).
func (rp *RepoPanel) RebuildFull() {
	rp.build()
}

func (rp *RepoPanel) rebuildList() {
	rp.list.Objects = nil
	rp.headerRows = rp.headerRows[:0]

	for gi, group := range rp.groups {
		// Compact visual separator between repo groups.
		if gi > 0 {
			gap := canvas.NewRectangle(colorPanelBg)
			gap.SetMinSize(fyne.NewSize(0, 4))
			sep := canvas.NewRectangle(colorBorder)
			sep.SetMinSize(fyne.NewSize(0, 1))
			rp.list.Add(gap)
			rp.list.Add(sep)
		}

		// Drop indicator: a colored bar showing where the dragged repo
		// would land. Drawn above the group at the drop target. When the
		// target is past the last group, indicator is drawn below it
		// (handled after the loop).
		if rp.dragSrc >= 0 && rp.dragDst == gi && rp.dragSrc != gi {
			rp.list.Add(rp.dropIndicator())
		}

		// Repo header — bold + foreground color for strong visual
		// hierarchy over the mode sub-items. The optional linked-worktree
		// count surfaces "tasks in flight" at a glance. truncMonoText
		// (instead of monoText) shrinks long repo names to fit, so a
		// narrow panel never triggers a horizontal scrollbar that would
		// push the chip off-screen.
		header := newTruncMonoText(group.Name, colorForeground, true, false)
		header.txt.TextSize = scaledSize(12)
		topGap := canvas.NewRectangle(colorPanelBg)
		topGap.SetMinSize(fyne.NewSize(0, 4))

		// Drag handle on the left of the header. Only the handle is
		// draggable so users don't accidentally reorder by clicking the
		// name area.
		handle := newDragHandle(gi, rp)

		var rightContent fyne.CanvasObject
		if group.LinkedWorktreeCount > 0 {
			rightContent = worktreeCountChip(group.LinkedWorktreeCount)
		}
		headerRow := container.NewBorder(nil, nil, handle, rightContent, header)

		groupHeader := container.NewVBox(topGap, headerRow)
		rp.headerRows = append(rp.headerRows, groupHeader)
		rp.list.Add(groupHeader)

		// Mode entries (clickable).
		for mi, mode := range group.Modes {
			groupIdx, modeIdx := gi, mi
			isActive := gi == rp.activeGrp && mi == rp.activeMode

			line := rp.buildModeLine(mode, isActive)
			tap := newTappableCard(line, func() {
				if rp.OnModeSelected != nil {
					rp.OnModeSelected(groupIdx, modeIdx)
				}
			})
			rp.list.Add(tap)
		}
	}

	// Drop indicator below the last group when dragging past the end.
	if rp.dragSrc >= 0 && rp.dragDst == len(rp.groups) && rp.dragSrc != len(rp.groups)-1 {
		rp.list.Add(rp.dropIndicator())
	}

	rp.list.Refresh()
}

func (rp *RepoPanel) dropIndicator() fyne.CanvasObject {
	bar := canvas.NewRectangle(colorSelected)
	bar.SetMinSize(fyne.NewSize(0, 2))
	return bar
}

func (rp *RepoPanel) buildModeLine(mode config.ModeEntry, isActive bool) fyne.CanvasObject {
	prefix := "  "
	if isActive {
		prefix = "▸ "
	}

	icon := "\U0001F4C2" // folder for regular
	if mode.Type == "sandbox" {
		icon = "\U0001F433" // whale for sandbox
	}

	modeLabel := "host"
	if mode.Agent != "" {
		modeLabel = mode.Agent
	}

	text := fmt.Sprintf("%s%s [%s]", prefix, icon, modeLabel)

	var labelColor = colorGray
	if isActive {
		labelColor = colorSelected
	}

	label := monoText(text, labelColor, isActive)
	label.TextSize = scaledSize(11)

	// Status dot for sandbox modes.
	var dot *canvas.Text
	if mode.Type == "sandbox" && mode.SandboxName != "" {
		if status, ok := rp.sbxStatuses[mode.SandboxName]; ok {
			dotText := " ●"
			var dotColor = colorRed
			switch status {
			case sandbox.StatusRunning:
				dotColor = colorGreen
			case sandbox.StatusStopped:
				dotColor = colorYellow
			}
			dot = monoText(dotText, dotColor, false)
		}
	}

	var row fyne.CanvasObject
	if dot != nil {
		row = container.NewHBox(label, dot)
	} else {
		row = label
	}

	// Active row gets a highlighted background so it stands out.
	if isActive {
		bg := canvas.NewRectangle(colorSelection)
		bg.CornerRadius = 4
		return container.NewStack(bg, container.NewPadded(row))
	}
	return row
}

// worktreeCountChip renders the linked-worktree count as a pill-shaped badge
// aligned to the right edge of the repo header. The trailing spacer keeps the
// chip away from the panel/scrollbar edge.
func worktreeCountChip(n int) fyne.CanvasObject {
	rightPad := canvas.NewRectangle(colorPanelBg)
	rightPad.SetMinSize(fyne.NewSize(8, 0))
	return container.NewHBox(countChip(n, colorSelection), rightPad)
}
