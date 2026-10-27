package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/mrzzmrzz/MySlack/locales"
)

// settings are the user's choices, kept as JSON in the app's data folder.
type settings struct {
	Appearance  int  `json:"appearance"` // 0 follows the system, 1 light, 2 dark
	TextSize    int  `json:"textSize"`   // 0 small, 1 standard, 2 large
	Compact     bool `json:"compact"`
	CmdEnter    bool `json:"cmdEnter"` // ⌘↩ sends, ↩ breaks the line
	Notify      bool `json:"notify"`
	OnlyMention bool `json:"onlyMention"` // notify of direct messages and mentions only
	Sound       bool `json:"sound"`
	DockBadge   bool `json:"dockBadge"`
}

func defaultSettings() settings {
	return settings{TextSize: 1, Notify: true, OnlyMention: true, Sound: true, DockBadge: true}
}

var textSizes = [...]float32{12, 13, 15}

func settingsPath() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "settings.json")
}

func loadSettings() settings {
	s := defaultSettings()
	if data, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(data, &s)
	}
	return s
}

func (a *app) saveSettings() {
	path := settingsPath()
	if path == "" {
		return
	}
	data, _ := json.MarshalIndent(a.settings, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("settings: %v", err)
	}
	if a.win != nil {
		a.win.Invalidate()
	}
	a.updateBadge()
}

// theme applies the appearance and text size chosen to the theme.
func (a *app) theme(c *ui.Context) *ui.Theme {
	var t ui.Theme
	switch a.settings.Appearance {
	case 1:
		t = *ui.LightTheme()
	case 2:
		t = *ui.DarkTheme()
	default:
		t = *c.Theme()
	}
	t.Accent, t.AccentText = c.Theme().Accent, c.Theme().AccentText
	t.Radius = 8
	t.FontSize = textSizes[a.settings.TextSize]
	return &t
}

// openSettings shows the settings window, as ⌘, does.
func (a *app) openSettings() {
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		return
	}
	a.cacheBytes = -1
	a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:           locales.SettingsTitle(),
		Width:           540,
		Height:          470,
		DisableResize:   true,
		DisableMinimize: true,
		DisableMaximize: true,
		TitleBarStyle:   mygo.TitleBarHidden,
		StateKey:        "settings",
		Content:         ui.View(a.settingsView),
	})
	a.settingsWin.OnClosed(func() { a.settingsWin = nil })
}

const (
	tabGeneral = iota
	tabNotifications
	tabAccount
)

var (
	gearIcon = icon(`<circle cx="8" cy="8" r="2.2"/><path d="M8 1.8v1.6M8 12.6v1.6M1.8 8h1.6M12.6 8h1.6M3.6 3.6l1.1 1.1M11.3 11.3l1.1 1.1M3.6 12.4l1.1-1.1M11.3 4.7l1.1-1.1"/>`)
	bellIcon = icon(`<path d="M4 11.5V7.2a4 4 0 0 1 8 0v4.3l1.2 1.3H2.8z"/><path d="M6.6 14a1.5 1.5 0 0 0 2.8 0"/>`)
)

// settingsView follows macOS's settings windows: tabs in the toolbar,
// and grouped rows of a label and its control, as System Settings shows
// them.
func (a *app) settingsView(c *ui.Context) {
	c.SetTheme(a.theme(c))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Height(titleBarHeight+24).Shrink(0).Justify(ui.Center).AlignItems(ui.End).Padding(0, 0, 8).DragWindow().Children(func() {
			ui.Row(c).Gap(2).Padding(3).Radius(14).Material(glass.Glass{}).Children(func() {
				for i, tab := range []struct {
					icon  *ui.SVG
					label string
				}{{gearIcon, locales.TabGeneral()}, {bellIcon, locales.TabNotifications()}, {personIcon, locales.TabAccount()}} {
					b := ui.ButtonBase(c.Key(tab.label)).Width(68).Padding(5, 0).Radius(11).Label(tab.label).Cursor(ui.CursorPointer)
					color := t.TextMuted
					switch {
					case a.settingsTab == i:
						b.Background(t.Accent.Alpha(0.14))
						color = t.Accent
					case b.Hovered():
						b.Background(t.SurfaceHover.Alpha(0.6))
					}
					b.Children(func() {
						ui.Column(c).Gap(2).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, tab.icon).Size(18, 18).TextColor(color)
							ui.Text(c, tab.label).FontSize(11).FontWeight(500).TextColor(color)
						})
					})
					if b.Clicked() {
						a.settingsTab = i
					}
				}
			})
		})
		ui.Scroll(c).Grow(1).Padding(8, 24, 24).Children(func() {
			ui.Column(c).Gap(18).Children(func() {
				switch a.settingsTab {
				case tabGeneral:
					a.generalSettings(c)
				case tabNotifications:
					a.notificationSettings(c)
				case tabAccount:
					a.accountSettings(c)
				}
			})
		})
	})
}

func (a *app) generalSettings(c *ui.Context) {
	s := &a.settings
	group(c, locales.Appearance(), func() {
		row(c, locales.Theme(), "", func() {
			if ui.Segmented(c, &s.Appearance, locales.ThemeSystem(), locales.ThemeLight(), locales.ThemeDark()).Label(locales.Theme()).Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, locales.TextSize(), "", func() {
			if ui.Segmented(c, &s.TextSize, locales.TextSmall(), locales.TextStandard(), locales.TextLarge()).Label(locales.TextSize()).Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, locales.Compact(), locales.CompactDetail(), func() {
			if ui.Switch(c, &s.Compact).Label(locales.Compact()).Changed() {
				a.saveSettings()
			}
		})
	})
	group(c, locales.Storage(), func() {
		used := locales.Calculating()
		switch {
		case a.cacheBytes == 0:
			used = locales.NoCache()
		case a.cacheBytes > 0:
			used = size(a.cacheBytes)
		}
		row(c, locales.Cache(), locales.CacheDetail(used), func() {
			if ui.Button(c, locales.ClearCache()).Disabled(a.cacheBytes <= 0).Clicked() {
				a.clearCache()
				a.cacheBytes = 0
				c.Toast(locales.CacheCleared())
			}
		})
	})
	a.measureCache()
	group(c, locales.Sending(), func() {
		row(c, locales.CmdEnter(), locales.CmdEnterDetail(), func() {
			if ui.Switch(c, &s.CmdEnter).Label(locales.CmdEnter()).Changed() {
				a.saveSettings()
			}
		})
	})
}

func (a *app) notificationSettings(c *ui.Context) {
	s := &a.settings
	group(c, locales.DesktopNotifications(), func() {
		row(c, locales.NotifyNew(), locales.NotifyNewDetail(), func() {
			if ui.Switch(c, &s.Notify).Label(locales.NotifyNew()).Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, locales.OnlyMentions(), "", func() {
			if ui.Switch(c, &s.OnlyMention).Label(locales.OnlyMentions()).Disabled(!s.Notify).Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, locales.Sound(), "", func() {
			if ui.Switch(c, &s.Sound).Label(locales.Sound()).Disabled(!s.Notify).Changed() {
				a.saveSettings()
			}
		})
	})
	group(c, locales.Dock(), func() {
		row(c, locales.DockBadge(), locales.DockBadgeDetail(), func() {
			if ui.Switch(c, &s.DockBadge).Label(locales.DockBadge()).Changed() {
				a.saveSettings()
			}
		})
	})
}

func (a *app) accountSettings(c *ui.Context) {
	t := c.Theme()
	if a.auth == nil {
		group(c, locales.TabAccount(), func() {
			row(c, locales.NotSignedIn(), locales.NotSignedInDetail(), func() {})
		})
		return
	}
	group(c, locales.TabAccount(), func() {
		ui.Row(c).Padding(12, 14).Gap(12).AlignItems(ui.Center).Children(func() {
			me := a.users[a.auth.UserID]
			name := a.auth.User
			var photo *ui.Bitmap
			if me != nil {
				name, photo = me.Label(), a.picture(me.Photo())
			}
			avatar(c, name, photo, 40)
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, name).FontWeight(600)
				ui.Text(c, a.auth.Team+" · "+a.auth.URL).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
		})
		separator(c)
		row(c, locales.Connection(), "", func() {
			state, color := locales.Connected(), ui.Hex("#2EB67D")
			if !a.connected {
				state, color = locales.Connecting(), t.Danger
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(color)
				ui.Text(c, state).TextColor(t.TextMuted)
			})
		})
	})
	group(c, "", func() {
		row(c, locales.SignOut(), locales.SignOutDetail(), func() {
			if ui.Button(c, locales.SignOutEllipsis()).Clicked() {
				a.signOut()
				if a.win != nil {
					a.win.Invalidate()
				}
			}
		})
	})
}

// group is a rounded section of rows under a title, as System Settings
// groups them.
func group(c *ui.Context, title string, fn func()) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(0, 6)
		}
		ui.Column(c).Radius(10).Background(t.Surface).Border(1, t.Border.Alpha(0.5)).Children(fn)
	})
}

// row is a setting: its label and a note on the left, its control on
// the right.
func row(c *ui.Context, label, note string, control func()) {
	t := c.Theme()
	ui.Row(c).Padding(10, 14).Gap(16).AlignItems(ui.Center).MinHeight(44).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, label)
			if note != "" {
				ui.Text(c, note).FontSize(11).TextColor(t.TextMuted)
			}
		})
		control()
	})
}

func separator(c *ui.Context) {
	ui.Box(c).Height(1).Margin(0, 14).Background(c.Theme().Border.Alpha(0.6))
}

// measureCache works out the cache's size for the settings, once each time
// they show it.
func (a *app) measureCache() {
	if a.cacheBytes >= 0 || a.measuring {
		return
	}
	a.measuring = true
	go func() {
		n := a.cacheSize()
		a.update(func() {
			a.cacheBytes, a.measuring = n, false
			if a.settingsWin != nil {
				a.settingsWin.Invalidate()
			}
		})
	}()
}
