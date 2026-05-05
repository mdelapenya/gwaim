package gui

import (
	"errors"
	"os"
	"os/exec"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/mdelapenya/biomelab/internal/config"
)

// setupSystemTray creates a system tray icon with Show/Hide toggle, Theme
// submenu, and Quit.
func (a *App) setupSystemTray() {
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return
	}

	toggleItem := fyne.NewMenuItem("Hide", nil)
	toggleItem.Action = func() {
		if a.window.Content().Visible() {
			a.window.Hide()
			toggleItem.Label = "Show"
		} else {
			a.window.Show()
			toggleItem.Label = "Hide"
		}
		desk.SetSystemTrayMenu(a.trayMenu)
	}

	// Theme submenu — Light / Dark with a checkmark on the active variant.
	a.trayThemeLight = fyne.NewMenuItem("Light", func() {
		a.applyThemeVariant(VariantLight)
	})
	a.trayThemeDark = fyne.NewMenuItem("Dark", func() {
		a.applyThemeVariant(VariantDark)
	})
	a.trayThemeLight.Checked = a.theme.Variant() == VariantLight
	a.trayThemeDark.Checked = a.theme.Variant() == VariantDark
	themeItem := fyne.NewMenuItem("Theme", nil)
	themeItem.ChildMenu = fyne.NewMenu("", a.trayThemeLight, a.trayThemeDark)

	configItem := fyne.NewMenuItem("Show Config", func() {
		if err := a.openConfigFile(); err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	a.trayMenu = fyne.NewMenu("biomelab",
		toggleItem,
		fyne.NewMenuItemSeparator(),
		themeItem,
		configItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			a.stopAllRefresh()
			a.fyneApp.Quit()
		}),
	)
	desk.SetSystemTrayMenu(a.trayMenu)
	desk.SetSystemTrayIcon(AppIcon)

	// Update label when window is hidden via close button.
	a.window.SetCloseIntercept(func() {
		a.window.Hide()
		toggleItem.Label = "Show"
		desk.SetSystemTrayMenu(a.trayMenu)
	})
}

// openConfigFile opens the config file with the system's default application.
// If the file does not exist yet (fresh install), an empty config is written
// first so the editor has something to open.
func (a *App) openConfigFile() error {
	if _, err := os.Stat(a.configPath); errors.Is(err, os.ErrNotExist) {
		if err := config.Save(a.configPath, &config.Config{}); err != nil {
			return err
		}
	}
	return openInSystem(a.configPath)
}

// openInSystem opens path with the OS default handler.
func openInSystem(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func (a *App) stopAllRefresh() {
	for _, re := range a.repos {
		if re.refreshMgr != nil {
			re.refreshMgr.Stop()
		}
	}
}
