package gui

import (
	"fmt"
	"image/color"
	"os/exec"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/mdelapenya/biomelab/internal/git"
	"github.com/mdelapenya/biomelab/internal/regent"
)

// regentLogWindowInitialSize is wide enough for typical prompts and full
// file paths inside tool args; the window is user-resizable.
var regentLogWindowInitialSize = fyne.NewSize(900, 640)

// regentLogLimit caps how many recent steps the window fetches. Matches
// rgt's own default; we surface a Refresh button rather than paging.
const regentLogLimit = 50

// showRegentLogModal opens (or reuses) a single top-level window showing
// `rgt log` for the given worktree. Subsequent presses of 'l' — same or
// different card — repopulate this same window rather than spawning new
// ones, so the user always has at most one regent log window open.
//
// Implementation: App keeps a single window handle plus a closure that
// re-renders content for a different worktree. When the window is closed,
// both are cleared so the next 'l' press creates a fresh one.
func (a *App) showRegentLogModal(wt git.Worktree) {
	if a.regentLogWindow != nil && a.regentLogReload != nil {
		a.regentLogReload(wt)
		a.regentLogWindow.RequestFocus()
		return
	}

	w := a.fyneApp.NewWindow("Regent activity — " + wt.Branch)
	a.regentLogWindow = w

	body := container.NewVBox()
	scroll := container.NewVScroll(body)

	current := wt
	render := func(target git.Worktree) {
		current = target
		w.SetTitle("Regent activity — " + target.Branch)
		body.Objects = a.buildRegentLogContent(target)
		body.Refresh()
	}
	render(wt)

	refresh := widget.NewButton("Refresh", func() { render(current) })
	refresh.Importance = widget.HighImportance
	exportBtn := widget.NewButton("Export JSON…", func() {
		a.exportRegentLog(current, w)
	})
	closeBtn := widget.NewButton("Close", func() { w.Close() })
	footer := container.NewBorder(nil, nil,
		container.NewHBox(refresh, exportBtn),
		closeBtn,
	)

	w.SetContent(container.NewBorder(nil, footer, nil, nil, scroll))
	w.Resize(regentLogWindowInitialSize)
	w.CenterOnScreen()

	a.regentLogReload = render
	w.SetOnClosed(func() {
		a.regentLogWindow = nil
		a.regentLogReload = nil
	})

	w.Show()
	w.RequestFocus()
}

// buildRegentLogContent returns the rendered children for the scrollable
// body: a session header followed by a step row per Step. Handles all
// "no data" states (rgt missing, no .regent/, no activity) with a
// friendly inline message.
func (a *App) buildRegentLogContent(wt git.Worktree) []fyne.CanvasObject {
	if _, err := exec.LookPath("rgt"); err != nil {
		// OS-agnostic message — install hints live in the deps dialog.
		return []fyne.CanvasObject{
			monoText("re_gent (rgt) is not installed.", colorYellow, true),
			monoText("See install instructions: https://github.com/regent-vcs/re_gent", colorDimGray, false),
			monoText("Or open the Dependencies dialog from the system tray.", colorDimGray, false),
		}
	}

	sessionID, steps, err := regent.LogJSON(wt.Path, regentLogLimit)
	if err != nil {
		return []fyne.CanvasObject{
			monoText("rgt log failed:", colorRed, true),
			monoText(err.Error(), colorRed, false),
		}
	}
	if len(steps) == 0 {
		return []fyne.CanvasObject{
			monoText("No regent activity yet.", colorGray, true),
			monoText("Run an agent in this worktree and the log will populate here.", colorDimGray, false),
		}
	}

	items := make([]fyne.CanvasObject, 0, len(steps)+1)
	header := monoText(fmt.Sprintf("Session %s · %d steps", sessionID, len(steps)), colorBranch, true)
	header.TextSize = scaledSize(11)
	items = append(items, header)
	items = append(items, widget.NewSeparator())

	for i, s := range steps {
		if i > 0 {
			items = append(items, widget.NewSeparator())
		}
		items = append(items, buildRegentStepRow(s))
	}
	return items
}

// buildRegentStepRow renders one Step as a stacked block: header line
// (sha · timestamp · origin), Human prompt, Agent reply, and a
// "[▶ N tools]" toggle button that reveals the tool list when clicked.
// The toggle is a button (not widget.Accordion, which misbehaves inside
// VScroll) — same proven pattern other surfaces in biomelab use.
func buildRegentStepRow(s Step) fyne.CanvasObject {
	rows := []fyne.CanvasObject{}

	// Header: short sha · timestamp · origin.
	headerText := fmt.Sprintf("● %s · %s", s.ShortHash(), s.Timestamp.Format("2006-01-02 15:04:05"))
	if s.Origin != "" {
		headerText += " · " + s.Origin
	}
	header := monoText(headerText, colorBranch, true)
	header.TextSize = scaledSize(11)
	rows = append(rows, header)

	if s.HumanPrompt != "" {
		rows = append(rows, roleLine("Human", s.HumanPrompt, colorBlue))
	}
	if s.AgentReply != "" {
		rows = append(rows, roleLine("Agent", s.AgentReply, colorGreen))
	}

	if len(s.Tools) > 0 {
		rows = append(rows, buildToolsCollapsible(s.Tools))
	}

	return container.NewVBox(rows...)
}

// roleLine renders a "Role: message" pair with the role bolded in its
// signature color and the message body underneath in word-wrapped Label
// form (so long prompts stay readable inside the window's width).
func roleLine(role, body string, c color.Color) fyne.CanvasObject {
	roleText := monoText(role+":", c, true)
	roleText.TextSize = scaledSize(11)

	msg := widget.NewLabel(body)
	msg.Wrapping = fyne.TextWrapWord
	return container.NewVBox(roleText, msg)
}

// buildToolsCollapsible renders a toggle button that expands/collapses
// the tools list under it. The list shows one row per tool with the
// tool name and its most important arg (file_path / command / query),
// then a key=value dump of any remaining args — all with full paths,
// no truncation.
func buildToolsCollapsible(tools []ToolCall) fyne.CanvasObject {
	count := len(tools)
	plural := "tool"
	if count != 1 {
		plural = "tools"
	}

	list := container.NewVBox()
	for _, t := range tools {
		list.Add(buildToolRow(t))
	}
	list.Hide()

	var toggle *widget.Button
	expanded := false
	toggle = widget.NewButton(fmt.Sprintf("▶ %d %s", count, plural), func() {
		expanded = !expanded
		if expanded {
			toggle.SetText(fmt.Sprintf("▼ %d %s", count, plural))
			list.Show()
		} else {
			toggle.SetText(fmt.Sprintf("▶ %d %s", count, plural))
			list.Hide()
		}
	})
	toggle.Alignment = widget.ButtonAlignLeading

	return container.NewVBox(toggle, list)
}

// buildToolRow renders one tool invocation: name in bold, primary arg
// inline next to it, secondary args one-per-line beneath.
func buildToolRow(t ToolCall) fyne.CanvasObject {
	primaryKey, primaryValue := pickPrimaryArg(t)

	head := monoText("└─ "+t.Name, colorPurple, true)
	head.TextSize = scaledSize(11)
	rows := []fyne.CanvasObject{}
	if primaryKey != "" {
		line := monoText(fmt.Sprintf("   %s: %s", primaryKey, primaryValue), colorDimGray, false)
		line.TextSize = scaledSize(10)
		rows = append(rows, container.NewVBox(head, line))
	} else {
		rows = append(rows, head)
	}

	// Secondary args (anything other than the primary key) on extra
	// lines, sorted for stable rendering.
	keys := make([]string, 0, len(t.Args))
	for k := range t.Args {
		if k == primaryKey {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		line := monoText(fmt.Sprintf("   %s: %s", k, formatArgValue(t.Args[k])), colorDimGray, false)
		line.TextSize = scaledSize(10)
		rows = append(rows, line)
	}
	return container.NewVBox(rows...)
}

// pickPrimaryArg chooses the most informative arg for a tool's primary
// display line: file_path for file ops, command for Bash, query for
// search-like tools. Falls back to "" so the caller can render just the
// tool name.
func pickPrimaryArg(t ToolCall) (string, string) {
	priority := []string{"file_path", "path", "command", "query", "url", "pattern", "prompt"}
	for _, k := range priority {
		if v, ok := t.Args[k]; ok {
			return k, formatArgValue(v)
		}
	}
	return "", ""
}

// formatArgValue renders an arg value as a single line. Strings pass
// through; everything else gets a short %v form. Newlines collapse to a
// single space so the line stays a line.
func formatArgValue(v any) string {
	s := fmt.Sprintf("%v", v)
	// Collapse newlines so multi-line args (commands, content blocks)
	// stay on one line in the row view. Users wanting full content can
	// click through to `rgt show <hash>` later.
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' {
			out = append(out, ' ')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// Step / ToolCall aliases so this file doesn't have to import the regent
// package's types inline at every signature. These mirror the public
// types one-to-one.
type (
	Step     = regent.Step
	ToolCall = regent.ToolCall
)

// exportRegentLog runs `rgt log --json` once more and writes the raw
// bytes to a user-chosen file. Errors surface via the dashboard's status
// line and dialog.ShowError so failures aren't silent. Empty results
// (no .regent/, no rgt) short-circuit with an informational dialog —
// there's nothing useful to write.
func (a *App) exportRegentLog(wt git.Worktree, parent fyne.Window) {
	data, err := regent.LogJSONRaw(wt.Path, regentLogLimit)
	if err != nil {
		dialog.ShowError(err, parent)
		return
	}
	if len(data) == 0 {
		dialog.ShowInformation("No data", "There is no regent activity to export for this worktree.", parent)
		return
	}

	defaultName := fmt.Sprintf("regent-log-%s-%s.json",
		sanitizeFilename(wt.Branch),
		time.Now().Format("20060102-150405"),
	)

	a.saveBytesNative(parent, defaultName, data)
}

// sanitizeFilename strips characters that are awkward in filenames
// (slashes, spaces) so a branch like "feat/new-thing" doesn't try to
// create nested directories on save.
func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch r {
		case '/', '\\', ' ', ':', '*', '?', '"', '<', '>', '|':
			out = append(out, '-')
		default:
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "branch"
	}
	return string(out)
}

