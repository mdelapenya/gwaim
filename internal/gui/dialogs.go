package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/mdelapenya/biomelab/internal/git"
	"github.com/mdelapenya/biomelab/internal/kits"
	"github.com/mdelapenya/biomelab/internal/provider"
	"github.com/mdelapenya/biomelab/internal/sandbox"
)

var dialogMinSize = fyne.NewSize(450, 200)
var kitsDialogSize = fyne.NewSize(600, 480)

// All dialog functions return the dialog so the caller can store it for Escape dismissal.

func showConfirmDelete(parent fyne.Window, branch string, onDone func(), onConfirm func()) dialog.Dialog {
	var d *dialog.ConfirmDialog
	msg := "Delete worktree '" + branch + "'?\n\nThis removes the directory, branch, and metadata."

	keyCap := newDialogKeyCapture(
		func() { d.Confirm() },
		func() { d.Hide() },
	)
	content := container.NewStack(widget.NewLabel(msg), keyCap)

	d = dialog.NewCustomConfirm("Delete Worktree", "Yes", "No", content, func(ok bool) {
		onDone()
		if ok {
			onConfirm()
		}
	}, parent)
	d.Resize(dialogMinSize)
	d.Show()
	focusInDialog(parent, keyCap)
	return d
}

func showConfirmCreateSandbox(parent fyne.Window, sbxName, sbxAgent, repoPath string, kitURLs []string, onDone func(), onConfirm func()) dialog.Dialog {
	var d *dialog.ConfirmDialog
	args := sandbox.CreateArgs(sbxName, sbxAgent, repoPath, kitURLs)
	cmd := sandbox.CommandString(args)

	keyCap := newDialogKeyCapture(
		func() { d.Confirm() },
		func() { d.Hide() },
	)
	body := container.NewVBox(
		widget.NewLabel("Create sandbox? This may take a few minutes."),
		widget.NewLabel("Command:"),
		monoText(cmd, colorSelected, false),
	)
	content := container.NewStack(body, keyCap)

	d = dialog.NewCustomConfirm("Create Sandbox", "Create", "Cancel", content, func(ok bool) {
		onDone()
		if ok {
			onConfirm()
		}
	}, parent)
	d.Resize(dialogMinSize)
	d.Show()
	focusInDialog(parent, keyCap)
	return d
}

// showConfirmRecreateSandbox is the destructive confirmation shown when the
// user wants to apply kits to an existing sandbox. Since `--kit` only works
// at create time, biomelab must `sbx rm --force` and re-create. Host worktrees
// are unaffected; container state is lost.
func showConfirmRecreateSandbox(parent fyne.Window, sbxName, sbxAgent, repoPath string, kitURLs []string, onDone func(), onConfirm func()) dialog.Dialog {
	var d *dialog.ConfirmDialog
	rmCmd := sandbox.CommandString(sandbox.RemoveArgs(sbxName))
	createCmd := sandbox.CommandString(sandbox.CreateArgs(sbxName, sbxAgent, repoPath, kitURLs))

	keyCap := newDialogKeyCapture(
		func() { d.Confirm() },
		func() { d.Hide() },
	)
	body := container.NewVBox(
		monoText("Recreate sandbox to apply kits?", colorYellow, true),
		widget.NewLabel("Container state will be lost. Host worktrees are preserved."),
		widget.NewSeparator(),
		widget.NewLabel("Commands:"),
		monoText(rmCmd, colorRed, false),
		monoText(createCmd, colorSelected, false),
	)
	content := container.NewStack(body, keyCap)

	d = dialog.NewCustomConfirm("Recreate Sandbox", "Recreate", "Cancel", content, func(ok bool) {
		onDone()
		if ok {
			onConfirm()
		}
	}, parent)
	d.Resize(kitsDialogSize)
	d.Show()
	focusInDialog(parent, keyCap)
	return d
}

func showConfirmRemoveSandbox(parent fyne.Window, sbxName string, onDone func(), onConfirm func()) dialog.Dialog {
	var d *dialog.ConfirmDialog
	args := sandbox.RemoveArgs(sbxName)
	cmd := sandbox.CommandString(args)

	keyCap := newDialogKeyCapture(
		func() { d.Confirm() },
		func() { d.Hide() },
	)
	body := container.NewVBox(
		widget.NewLabel("Remove sandbox? This stops and deletes all containers."),
		widget.NewLabel("Command:"),
		monoText(cmd, colorRed, false),
	)
	content := container.NewStack(body, keyCap)

	d = dialog.NewCustomConfirm("Remove Sandbox", "Remove", "Cancel", content, func(ok bool) {
		onDone()
		if ok {
			onConfirm()
		}
	}, parent)
	d.Resize(dialogMinSize)
	d.Show()
	focusInDialog(parent, keyCap)
	return d
}

func showConfirmRemoveMode(parent fyne.Window, repoName, modeLabel string, isSandbox bool, onDone func(), onConfirm func()) dialog.Dialog {
	var d *dialog.ConfirmDialog
	var msg string
	if isSandbox {
		msg = fmt.Sprintf("Remove sandbox mode '%s' from %s?\n\nThe sandbox itself is not deleted.", modeLabel, repoName)
	} else {
		msg = fmt.Sprintf("Remove '%s' from %s?", modeLabel, repoName)
	}

	keyCap := newDialogKeyCapture(
		func() { d.Confirm() },
		func() { d.Hide() },
	)
	content := container.NewStack(widget.NewLabel(msg), keyCap)

	d = dialog.NewCustomConfirm("Remove Mode", "Yes", "No", content, func(ok bool) {
		onDone()
		if ok {
			onConfirm()
		}
	}, parent)
	d.Resize(dialogMinSize)
	d.Show()
	focusInDialog(parent, keyCap)
	return d
}

// --- Send PR dialogs ---

func showSendPRDirtyWarning(parent fyne.Window, branch string, dirty, hasStash bool, onDone func(), onProceed func()) dialog.Dialog {
	var d *dialog.ConfirmDialog
	var warnings []string
	if dirty {
		warnings = append(warnings, "Branch has uncommitted changes.")
	}
	if hasStash {
		warnings = append(warnings, "Branch has stashed changes.")
	}

	body := container.NewVBox(
		monoText("Send PR for: "+branch, colorBranch, true),
		widget.NewSeparator(),
	)
	for _, w := range warnings {
		body.Add(monoText("⚠ "+w, colorYellow, false))
	}
	body.Add(widget.NewLabel("\nProceed anyway?"))

	keyCap := newDialogKeyCapture(
		func() { d.Confirm() },
		func() { d.Hide() },
	)
	content := container.NewStack(body, keyCap)

	d = dialog.NewCustomConfirm("Uncommitted Changes", "Continue", "Cancel", content, func(ok bool) {
		if !ok {
			onDone()
			return
		}
		onProceed()
	}, parent)
	d.Resize(dialogMinSize)
	d.Show()
	focusInDialog(parent, keyCap)
	return d
}

func showSendPRRemoteSelection(parent fyne.Window, remotes []git.RemoteInfo, onDone func(), onSelect func(idx int)) dialog.Dialog {
	var d dialog.Dialog

	content := container.NewVBox(
		widget.NewLabel("Select a remote to push to:"),
		widget.NewSeparator(),
	)

	var firstBtn *dialogButton
	for i, r := range remotes {
		idx := i
		label := fmt.Sprintf("%s  (%s)", r.Name, r.Repo)
		btn := newDialogButton(label, func() {
			d.Hide()
			onSelect(idx)
		}, func() { d.Hide() })
		if firstBtn == nil {
			firstBtn = btn
		}
		content.Add(btn)
	}

	d = dialog.NewCustom("Select Remote", "Cancel", content, parent)
	d.SetOnClosed(func() {
		onDone()
	})
	d.Resize(dialogMinSize)
	d.Show()
	if firstBtn != nil {
		focusInDialog(parent, firstBtn)
	}
	return d
}

// showKitsDialog renders a multi-select picker for sandbox kits, split into
// Agents and Mixins. Mixins are pre-filtered for the sandbox's agent by the
// caller — this function only renders.
//
// onSubmit fires with the selected kits in the order they were rendered.
func showKitsDialog(
	parent fyne.Window,
	sandboxName, agent string,
	agents, mixins []kits.Kit,
	onDone func(),
	onSubmit func(selected []kits.Kit),
) dialog.Dialog {
	var d *dialog.ConfirmDialog
	checks := make([]*widget.Check, 0, len(agents)+len(mixins))
	all := make([]kits.Kit, 0, len(agents)+len(mixins))

	body := container.NewVBox(
		monoText(
			fmt.Sprintf("Install kits into %s (agent: %s)", sandboxName, agent),
			colorBranch, true,
		),
		widget.NewSeparator(),
	)

	addSection := func(title string, list []kits.Kit, emptyHint string) {
		header := monoText(title, colorBlue, true)
		body.Add(header)
		if len(list) == 0 {
			hint := monoText(emptyHint, colorDimGray, false)
			hint.TextSize = scaledSize(11)
			body.Add(hint)
			body.Add(widget.NewSeparator())
			return
		}
		for _, k := range list {
			checks = append(checks, widget.NewCheck(displayLabel(k), nil))
			all = append(all, k)
			body.Add(checks[len(checks)-1])
			if k.Description != "" {
				desc := monoText("    "+k.Description, colorGray, false)
				desc.TextSize = scaledSize(10)
				body.Add(desc)
			}
		}
		body.Add(widget.NewSeparator())
	}

	addSection("Agents", agents, "(none available)")
	addSection(fmt.Sprintf("Mixins for %s", agent), mixins, "(no mixins compatible with this agent)")

	// keyCap captures Escape (cancel) globally inside the dialog. Enter is
	// NOT bound to confirm because Space toggles the focused checkbox and
	// users may want to toggle several before submitting — Enter would
	// race with that. They click "Install" or Tab to it and press Space.
	keyCap := newDialogKeyCapture(
		func() {}, // no-op on Enter — see comment above
		func() { d.Hide() },
	)
	scroll := container.NewVScroll(body)
	scroll.SetMinSize(kitsDialogSize)
	content := container.NewStack(scroll, keyCap)

	d = dialog.NewCustomConfirm("Install Kits", "Install", "Cancel", content, func(ok bool) {
		onDone()
		if !ok {
			return
		}
		var selected []kits.Kit
		for i, c := range checks {
			if c.Checked {
				selected = append(selected, all[i])
			}
		}
		if len(selected) == 0 {
			return
		}
		onSubmit(selected)
	}, parent)
	d.Resize(kitsDialogSize)
	d.Show()
	// Focus the first checkbox if any exist, so the user can Space-toggle
	// immediately. Fall back to the key-capture overlay so Escape still
	// dismisses even with no checkboxes.
	if len(checks) > 0 {
		focusInDialog(parent, checks[0])
	} else {
		focusInDialog(parent, keyCap)
	}
	return d
}

// displayLabel returns the user-visible label for a kit row.
func displayLabel(k kits.Kit) string {
	name := k.DisplayName
	if name == "" {
		name = k.Name
	}
	return fmt.Sprintf("%s  (%s)", name, k.Name)
}

func showSendPRConfirm(parent fyne.Window, branch string, remote git.RemoteInfo, existingPR *provider.PRInfo, onDone func(), onConfirm func()) dialog.Dialog {
	var d *dialog.ConfirmDialog
	var title, action string
	body := container.NewVBox()

	if existingPR != nil {
		title = "Push Commits"
		action = "Push"
		body.Add(monoText(fmt.Sprintf("PR #%d already exists: %s", existingPR.Number, existingPR.Title), colorBlue, false))
		body.Add(widget.NewSeparator())
		body.Add(widget.NewLabel("Push new commits to update?"))
	} else {
		title = "Create Pull Request"
		action = "Create PR"
		body.Add(widget.NewLabel("Create a new PR:"))
	}

	body.Add(widget.NewSeparator())
	body.Add(monoText("Branch: "+branch, colorBranch, true))
	body.Add(monoText("Remote: "+remote.Name+" ("+remote.Repo+")", colorGray, false))

	keyCap := newDialogKeyCapture(
		func() { d.Confirm() },
		func() { d.Hide() },
	)
	content := container.NewStack(body, keyCap)

	d = dialog.NewCustomConfirm(title, action, "Cancel", content, func(ok bool) {
		onDone()
		if ok {
			onConfirm()
		}
	}, parent)
	d.Resize(dialogMinSize)
	d.Show()
	focusInDialog(parent, keyCap)
	return d
}
