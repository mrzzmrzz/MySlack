package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
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
		Title:           "设置",
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
				}{{gearIcon, "通用"}, {bellIcon, "通知"}, {personIcon, "账户"}} {
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
	group(c, "外观", func() {
		row(c, "主题", "", func() {
			if ui.Segmented(c, &s.Appearance, "跟随系统", "浅色", "深色").Label("主题").Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, "文字大小", "", func() {
			if ui.Segmented(c, &s.TextSize, "小", "标准", "大").Label("文字大小").Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, "紧凑模式", "减小消息间距和头像尺寸", func() {
			if ui.Switch(c, &s.Compact).Label("紧凑模式").Changed() {
				a.saveSettings()
			}
		})
	})
	group(c, "存储", func() {
		used := "正在计算…"
		switch {
		case a.cacheBytes == 0:
			used = "没有缓存"
		case a.cacheBytes > 0:
			used = size(a.cacheBytes)
		}
		row(c, "缓存", "头像、缩略图和预览过的文件，"+used, func() {
			if ui.Button(c, "清除缓存").Disabled(a.cacheBytes <= 0).Clicked() {
				a.clearCache()
				a.cacheBytes = 0
				c.Toast("已清除缓存")
			}
		})
	})
	a.measureCache()
	group(c, "发送消息", func() {
		row(c, "用 ⌘↩ 发送", "打开后 ↩ 换行；关闭时 ↩ 发送、⇧↩ 换行", func() {
			if ui.Switch(c, &s.CmdEnter).Label("用 ⌘↩ 发送").Changed() {
				a.saveSettings()
			}
		})
	})
}

func (a *app) notificationSettings(c *ui.Context) {
	s := &a.settings
	group(c, "桌面通知", func() {
		row(c, "新消息通知", "窗口不在前台或不在当前会话时提醒", func() {
			if ui.Switch(c, &s.Notify).Label("新消息通知").Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, "仅私信和 @提及", "", func() {
			if ui.Switch(c, &s.OnlyMention).Label("仅私信和 @提及").Disabled(!s.Notify).Changed() {
				a.saveSettings()
			}
		})
		separator(c)
		row(c, "声音", "", func() {
			if ui.Switch(c, &s.Sound).Label("声音").Disabled(!s.Notify).Changed() {
				a.saveSettings()
			}
		})
	})
	group(c, "程序坞", func() {
		row(c, "显示未读数", "在程序坞图标上显示有未读消息的会话数", func() {
			if ui.Switch(c, &s.DockBadge).Label("显示未读数").Changed() {
				a.saveSettings()
			}
		})
	})
}

func (a *app) accountSettings(c *ui.Context) {
	t := c.Theme()
	if a.auth == nil {
		group(c, "账户", func() {
			row(c, "未登录", "在主窗口登录 Slack", func() {})
		})
		return
	}
	group(c, "账户", func() {
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
		row(c, "连接", "", func() {
			state, color := "已连接", ui.Hex("#2EB67D")
			if !a.connected {
				state, color = "正在连接…", t.Danger
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(color)
				ui.Text(c, state).TextColor(t.TextMuted)
			})
		})
	})
	group(c, "", func() {
		row(c, "退出登录", "从钥匙串删除这个 workspace 的会话", func() {
			if ui.Button(c, "退出登录…").Clicked() {
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
