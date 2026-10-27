package main

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/mrzzmrzz/Slack-MyGo/slack"
)

// The window follows macOS 27: a sidebar of the system's material along
// the window's whole edge, the traffic lights over it; a toolbar of
// frosted glass across the content, which the messages scroll under; a
// thread as an inspector beside them; and a composer of glass floating
// over their end.
const (
	titleBarHeight = 52
	sidebarWidth   = 248
	threadWidth    = 380
	composerRoom   = 84 // what the composer covers at the end of the messages
)

//go:embed resources/icon.png
var iconPNG []byte

var appIcon, _ = ui.DecodeBitmap(iconPNG)

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

// Icons in the manner of SF Symbols.
var (
	hashIcon    = icon(`<path d="M6.2 2.5 5 13.5M11 2.5 9.8 13.5M2.8 5.8h11M2.2 10.2h11"/>`)
	lockIcon    = icon(`<rect x="3.5" y="7" width="9" height="6.5" rx="1.8"/><path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2"/>`)
	personIcon  = icon(`<circle cx="8" cy="5.2" r="2.6"/><path d="M3 13.5c.6-2.6 2.6-4 5-4s4.4 1.4 5 4"/>`)
	peopleIcon  = icon(`<circle cx="6" cy="5.5" r="2.2"/><path d="M1.8 13c.5-2.3 2.2-3.5 4.2-3.5s3.7 1.2 4.2 3.5"/><path d="M10.2 3.6a2.2 2.2 0 0 1 0 4.1M11.6 9.7c1.4.4 2.3 1.6 2.6 3.3"/>`)
	sendIcon    = icon(`<path d="M8 13V3.5M4 7.2 8 3.2l4 4"/>`)
	closeIcon   = icon(`<path d="M4.5 4.5l7 7M11.5 4.5l-7 7"/>`)
	logoutIcon  = icon(`<path d="M9.5 3H4.5A1.5 1.5 0 0 0 3 4.5v7A1.5 1.5 0 0 0 4.5 13h5"/><path d="M7 8h7M11.5 5.5 14 8l-2.5 2.5"/>`)
	bubbleIcon  = icon(`<path d="M3.5 3h9A1.5 1.5 0 0 1 14 4.5v5.5a1.5 1.5 0 0 1-1.5 1.5H7l-3 2.5v-2.5h-.5A1.5 1.5 0 0 1 2 10V4.5A1.5 1.5 0 0 1 3.5 3z"/>`)
	sidebarIcon = icon(`<rect x="1.8" y="2.8" width="12.4" height="10.4" rx="2.2"/><path d="M6 2.8v10.4M3.4 5.4h1M3.4 7.4h1"/>`)
	threadsIcon = icon(`<path d="M2.5 3h11M2.5 6.5h7M5 10h8.5M5 13.5h5.5"/>`)
)

func (a *app) view(c *ui.Context) {
	a.services = c.Services()
	c.SetTheme(a.theme(c))
	t := c.Theme()
	if a.auth == nil {
		a.signInView(c)
		return
	}
	// The window's material shows where the view draws no background:
	// under the sidebar. Without it, as in tests, the sidebar draws one.
	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	}
	bar := c.TitleBar()
	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	shown := row.Animate("sidebar", b2f(!a.sidebarHidden), 250*time.Millisecond)
	// The toolbar's title keeps clear of the traffic lights and the
	// sidebar's button while the sidebar hides.
	a.titleInset = 20 + (1-shown)*(bar.Left+36)
	row.Children(func() {
		// The sidebar slides out to the left as it hides, as AppKit's.
		w := sidebarWidth * shown
		ui.Row(c).Width(w).Shrink(0).AlignItems(ui.Stretch).Clip().Children(func() {
			side := ui.Column(c).Width(sidebarWidth).Shrink(0).Margin(0, 0, 0, w-sidebarWidth)
			if !vibrant {
				side.Background(t.Surface)
			}
			side.Children(func() { a.sidebar(c) })
		})
		if w > 0 {
			ui.Box(c).Width(1).Background(t.Border.Alpha(0.7))
		}
		ui.Box(c).Grow(1).MinWidth(0).Background(t.Background).Children(func() { a.content(c) })
		if a.thread != nil && a.thread.channel == a.selected {
			ui.Box(c).Width(1).Background(t.Border.Alpha(0.7))
			a.threadPane(c)
		}
	})

	// The sidebar's button sits beside the traffic lights, whether the
	// sidebar shows or not.
	label := "隐藏边栏"
	if a.sidebarHidden {
		label = "显示边栏"
	}
	toggle := a.toolbarButton(c.Key("toggle-sidebar"), sidebarIcon, label).Absolute().
		Left(bar.Left + 6).Top((titleBarHeight - 28) / 2)
	if toggle.Clicked() {
		a.sidebarHidden = !a.sidebarHidden
	}
	if a.thread != nil {
		c.OnShortcut(0, ui.KeyEscape, func() { a.thread = nil })
	}
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// Sign-in

func (a *app) signInView(c *ui.Context) {
	t := c.Theme()
	ui.Box(c).Fill().Background(t.Background).Children(func() {
		// Colors of the logo behind the glass, which it bends and frosts,
		// around the middle of the window.
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
			ui.Box(c).Size(1, 1).Children(func() {
				for _, b := range []struct {
					x, y, r float32
					color   string
				}{{-170, -150, 190, "#36C5F0"}, {170, -130, 170, "#2EB67D"}, {-160, 160, 180, "#E01E5A"}, {170, 150, 200, "#ECB22E"}} {
					ui.Box(c).Absolute().Left(b.x-b.r).Top(b.y-b.r).Size(2*b.r, 2*b.r).Radius(b.r).
						Background(ui.Hex(b.color).Alpha(0.55)).PassThrough()
				}
			})
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Material(glass.Blur{Radius: 40})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(titleBarHeight).DragWindow()
		ui.Column(c).Fill().Center().Children(func() {
			ui.Column(c).Width(400).Padding(32, 32, 28).Gap(14).AlignItems(ui.Center).Radius(26).
				Material(glass.Glass{}).Children(func() {
				ui.Image(c, appIcon).Size(72, 72)
				ui.Text(c, "登录 Slack").FontSize(22).FontWeight(700).Margin(4, 0, 0, 0)
				ui.Text(c, "运行 go run ./cmd/import-safari 从 Safari 导入，或填入网页版 Slack 的 xoxc token 和 d cookie。").
					FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center).LineHeight(1.4)
				ui.Column(c).Gap(8).Width(336).Margin(6, 0, 0, 0).Children(func() {
					ui.TextInput(c, &a.tokenIn).Label("Token").Placeholder("xoxc-…").Password().Radius(10)
					cookie := ui.TextInput(c, &a.cookieIn).Label("d cookie").Placeholder("xoxd-…").Password().Radius(10)
					ready := !a.signingIn && strings.HasPrefix(strings.TrimSpace(a.tokenIn), "xox") && strings.TrimSpace(a.cookieIn) != ""
					if cookie.Submitted() && ready {
						a.signIn(a.tokenIn, a.cookieIn, true)
					}
					label := "登录"
					if a.signingIn {
						label = "正在登录…"
					}
					button := ui.ButtonBase(c).Height(34).Radius(17).Center().Margin(6, 0, 0, 0).Label(label).Disabled(!ready)
					if ready {
						button.Material(glass.Glass{Tint: t.Accent, Interactive: true}).Cursor(ui.CursorPointer)
					} else {
						button.Background(t.Surface)
					}
					button.Children(func() {
						color := t.AccentText
						if !ready {
							color = t.TextMuted
						}
						ui.Text(c, label).FontWeight(600).TextColor(color)
					})
					if button.Clicked() && ready {
						a.signIn(a.tokenIn, a.cookieIn, true)
					}
				})
				if a.signInErr != "" {
					ui.Text(c, a.signInErr).FontSize(12).TextColor(t.Danger).Selectable().TextAlign(ui.Center)
				}
			})
		})
	})
}

// Sidebar

// sidebar is the conversations under the traffic lights, and the user
// at its foot.
func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	var rooms, dms []*channel
	for _, ch := range a.channels {
		if ch.IsIM || ch.IsMPIM {
			if ch.IsIM && a.users[ch.User] != nil && a.users[ch.User].Deleted {
				continue
			}
			dms = append(dms, ch)
		} else {
			rooms = append(rooms, ch)
		}
	}
	byTitle := func(x, y *channel) int {
		return strings.Compare(strings.ToLower(a.label(x)), strings.ToLower(a.label(y)))
	}
	slices.SortFunc(rooms, byTitle)
	slices.SortFunc(dms, byTitle)

	ui.Box(c).Height(titleBarHeight).Shrink(0).DragWindow()
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		if len(a.channels) == 0 {
			ui.Column(c).Grow(1).Center().Children(func() {
				if a.connErr != "" {
					ui.Text(c, a.connErr).FontSize(12).TextColor(t.Danger).Padding(12)
				} else {
					ui.Spinner(c)
				}
			})
		} else {
			item := func(ch *channel) {
				// People show by their faces, as in Messages, and channels
				// by a glyph on a colored circle, as Reminders' lists: the
				// item's own icon is only an SVG, so both sit in its room.
				it := ui.SidebarItem(c, ch.ID, nil, a.label(ch)).Padding(t.Space(2), t.Space(2), t.Space(2), 33)
				it.Children(func() {
					ui.Row(c).Absolute().Left(6).Top(0).Bottom(0).Width(22).AlignItems(ui.Center).PassThrough().Children(func() {
						if ch.IsIM || ch.IsMPIM {
							a.faces(c, ch)
						} else {
							channelBadge(c, ch, 22)
						}
					})
					if ch.unread {
						ui.Box(c).Size(8, 8).Radius(4).Background(t.Accent)
					}
				})
			}
			list := ui.Sidebar(c, &a.selected, func() {
				ui.SidebarSection(c, "频道", &a.channelsOpen, func() {
					for _, ch := range rooms {
						item(ch)
					}
				})
				ui.SidebarSection(c, "私信", &a.dmsOpen, func() {
					for _, ch := range dms {
						item(ch)
					}
				})
			}).Grow(1).Background(ui.Transparent).Label("会话")
			if list.Changed() {
				a.open(a.selected)
			}
		}
	})
	a.account(c)
}

// account is the user at the foot of the sidebar, a button whose menu,
// the system's, opens the settings or signs out.
func (a *app) account(c *ui.Context) {
	t := c.Theme()
	me := a.users[a.auth.UserID]
	name := a.auth.User
	var photo *ui.Bitmap
	if me != nil {
		name, photo = me.Label(), a.picture(me.Photo())
	}
	// Its avatar lines up with those of the direct messages above.
	b := ui.ButtonBase(c.Key("account")).AlignSelf(ui.Stretch).Justify(ui.Start).Margin(6, 8, 8).Padding(6, 8, 6, 6).Radius(8).Label(name)
	if b.Hovered() {
		b.Background(t.SurfaceHover.Alpha(0.6))
	}
	b.Children(func() {
		ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
			avatar(c, name, photo, 26)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, name).FontWeight(600).SingleLine()
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					status, state := ui.Hex("#34C759"), a.auth.Team
					if !a.connected {
						status, state = ui.Hex("#FF9F0A"), "正在连接…"
					}
					ui.Box(c).Size(7, 7).Radius(3.5).Background(status)
					ui.Text(c, state).FontSize(11).TextColor(t.TextMuted).SingleLine()
				})
			})
			ui.Icon(c, chevronIcon).Size(12, 12).TextColor(t.TextMuted)
		})
	})
	b.Menu(func(m *ui.Menu) {
		if m.Item("设置…").Chosen() {
			a.openSettings()
		}
		m.Separator()
		if m.Item("退出登录").Chosen() {
			a.signOut()
		}
	})
}

var chevronIcon = icon(`<path d="M5 6.5 8 3.5l3 3M5 9.5l3 3 3-3"/>`)

// toolbarButton is a borderless button holding an icon, as in macOS's
// sidebars and inspectors.
func (a *app) toolbarButton(c *ui.Context, svg *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(28, 28).Radius(7).Center().Label(label).Tooltip(label).TextColor(t.TextMuted)
	if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.Children(func() { ui.Icon(c, svg).Size(16, 16) })
	return b
}

// glassButton is a round button of glass holding an icon.
func (a *app) glassButton(c *ui.Context, svg *ui.SVG, label string, size float32) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(size, size).Radius(size / 2).Center().Label(label).Tooltip(label).
		Cursor(ui.CursorPointer).Material(glass.Glass{Interactive: true}).TextColor(t.Text)
	b.Children(func() { ui.Icon(c, svg).Size(size*0.5, size*0.5) })
	return b
}

// faces is the avatar of a direct message's person, or two overlapping
// of a group's, as Messages shows them.
func (a *app) faces(c *ui.Context, ch *channel) {
	if ch.IsIM {
		name, photo := ch.User, (*ui.Bitmap)(nil)
		if u := a.users[ch.User]; u != nil {
			name, photo = u.Label(), a.picture(u.Photo())
		}
		avatar(c, name, photo, 22)
		return
	}
	names := strings.Split(a.title(ch), ", ")
	ui.Box(c).Size(22, 22).Children(func() {
		avatar(c, names[0], nil, 15).Absolute().Left(0).Top(0)
		if len(names) > 1 {
			avatar(c, names[1], nil, 15).Absolute().Right(0).Bottom(0)
		}
	})
}

// systemColors are macOS's system colors, a channel's taken by its ID.
var systemColors = []string{"#0A84FF", "#5E5CE6", "#BF5AF2", "#FF375F", "#FF9F0A", "#30D158", "#30B0C7", "#AC8E68"}

// channelBadge is a channel's glyph, white on a circle of its color,
// lighter at the top as the avatars are.
func channelBadge(c *ui.Context, ch *channel, size float32) {
	h := uint32(2166136261)
	for i := 0; i < len(ch.ID); i++ {
		h = (h ^ uint32(ch.ID[i])) * 16777619
	}
	base := ui.Hex(systemColors[h%uint32(len(systemColors))])
	glyph := boldHashIcon
	if ch.IsPrivate {
		glyph = boldLockIcon
	}
	ui.Box(c).Size(size, size).Radius(size/2).Shrink(0).Center().
		Gradient(base.Mix(ui.RGB(255, 255, 255), 0.12), base, 180).Children(func() {
		ui.Icon(c, glyph).Size(size*0.55, size*0.55).TextColor(ui.RGB(255, 255, 255))
	})
}

// Glyphs heavy enough to read white on a small circle.
var (
	boldHashIcon = boldIcon(`<path d="M6.2 2.5 5 13.5M11 2.5 9.8 13.5M2.8 5.8h11M2.2 10.2h11"/>`)
	boldLockIcon = boldIcon(`<rect x="3.2" y="7" width="9.6" height="6.8" rx="1.6" fill="currentColor"/><path d="M5.4 7V5.2a2.6 2.6 0 0 1 5.2 0V7"/>`)
)

func boldIcon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

func (a *app) channelIcon(ch *channel) *ui.SVG {
	switch {
	case ch.IsIM:
		return personIcon
	case ch.IsMPIM:
		return peopleIcon
	case ch.IsPrivate:
		return lockIcon
	}
	return hashIcon
}

// label is a conversation's name without the mark of its kind, which its
// icon shows.
func (a *app) label(ch *channel) string {
	if ch.IsIM || ch.IsMPIM {
		return a.title(ch)
	}
	return ch.Name
}

// Messages

func (a *app) content(c *ui.Context) {
	t := c.Theme()
	ch := a.channels[a.selected]
	if ch == nil {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(titleBarHeight).DragWindow()
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			ui.Icon(c, bubbleIcon).Size(40, 40).TextColor(t.TextMuted.Alpha(0.6))
			ui.Text(c, "选择一个频道或私信").FontSize(15).TextColor(t.TextMuted)
		})
		return
	}
	ch.list.Key = func(i int) any { return ch.messages[i].TS }
	ch.list.FollowEnd = true
	ui.List(c.Key("list:"+ch.ID), &ch.list, len(ch.messages), func(i int) {
		a.message(c, ch, ch.messages, i, true)
	}).Fill().Padding(titleBarHeight+4, 0, composerRoom, 0).ScrollbarInsets(titleBarHeight, 0, composerRoom, 0).
		Justify(ui.End).Children(func() {
		switch {
		case ch.err != "":
			ui.Text(c, ch.err).TextColor(t.Danger).Margin(ui.Auto)
		case ch.loading:
			ui.Spinner(c).Margin(ui.Auto)
		case len(ch.messages) == 0:
			ui.Text(c, "还没有消息").TextColor(t.TextMuted).Margin(ui.Auto)
		}
	})

	// The toolbar: the messages frost under it, as under macOS 27's.
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(titleBarHeight).PassThrough().Material(glass.ScrollEdge{Hard: true})
	ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(titleBarHeight).Padding(0, 16, 0, a.titleInset).Gap(10).
		AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			name := a.label(ch)
			if !ch.IsIM && !ch.IsMPIM {
				name = "# " + name
			}
			ui.Text(c, name).FontSize(14).FontWeight(600).SingleLine()
			topic := ch.Topic.Value
			if topic == "" {
				topic = ch.Purpose.Value
			}
			if topic != "" {
				ui.Text(c, slack.PlainText(topic, a.name)).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
	})

	// The composer floats over the end of the messages.
	ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(composerRoom + 10).PassThrough().Material(glass.ScrollEdge{Bottom: true})
	ui.Box(c).Absolute().Bottom(14).Left(20).Right(20).Children(func() {
		a.composer(c.Key("composer:"+ch.ID), &a.draft, "发消息到 "+a.title(ch), func(text string) { a.send(ch.ID, text, "") })
	})
}

// threadPane is the open thread, an inspector at the right.
func (a *app) threadPane(c *ui.Context) {
	t := c.Theme()
	th := a.thread
	ch := a.channels[th.channel]
	ui.Box(c).Width(threadWidth).Shrink(0).Background(t.Background).Children(func() {
		th.list.Key = func(i int) any { return th.messages[i].TS }
		th.list.FollowEnd = true
		ui.List(c.Key("thread:"+th.ts), &th.list, len(th.messages), func(i int) {
			a.message(c, ch, th.messages, i, false)
		}).Fill().Padding(titleBarHeight, 0, composerRoom, 0).ScrollbarInsets(titleBarHeight, 0, composerRoom, 0).Children(func() {
			switch {
			case th.err != "":
				ui.Text(c, th.err).TextColor(t.Danger).Margin(ui.Auto)
			case th.loading:
				ui.Spinner(c).Margin(ui.Auto)
			}
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(titleBarHeight).PassThrough().Material(glass.ScrollEdge{Hard: true})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(titleBarHeight).Padding(0, 12, 0, 18).Gap(8).
			AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Icon(c, threadsIcon).Size(15, 15).TextColor(t.TextMuted)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, "线程").FontSize(14).FontWeight(600)
				ui.Text(c, a.title(ch)).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
			if a.toolbarButton(c.Key("close-thread"), closeIcon, "关闭线程").Clicked() {
				a.thread = nil
			}
		})
		ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(composerRoom).PassThrough().Material(glass.ScrollEdge{Bottom: true})
		ui.Box(c).Absolute().Bottom(14).Left(14).Right(14).Children(func() {
			a.composer(c.Key("composer:"+th.ts), &th.draft, "回复…", func(text string) { a.send(th.channel, text, th.ts) })
		})
	})
}

// message builds the i-th of ms as Slack does: avatar, name and time,
// then the text, pictures, reactions and replies. A message following
// one of the same author closely leaves out the first three.
func (a *app) message(c *ui.Context, ch *channel, ms []slack.Message, i int, threads bool) {
	t := c.Theme()
	m := &ms[i]
	newDay := i == 0 || !sameDay(ms[i-1].Time(), m.Time())
	grouped := !newDay && ms[i-1].User == m.User && m.User != "" && !notice(&ms[i-1]) &&
		m.Time().Sub(ms[i-1].Time()) < 5*time.Minute
	ui.Column(c).Children(func() {
		if newDay {
			// The day, as Messages heads its conversations.
			ui.Row(c).Justify(ui.Center).Padding(18, 0, 6).Children(func() {
				ui.Text(c, day(m.Time())).FontSize(11).FontWeight(600).TextColor(t.TextMuted)
			})
		}
		if notice(m) {
			// Joins, topics and the like read as a line of their own.
			ui.Row(c).Margin(0, 8).Padding(3, 12, 3, 12+avatarSize(a.settings.Compact)+10).Gap(6).AlignItems(ui.Baseline).Children(func() {
				ui.Text(c, strings.TrimPrefix(slack.PlainText(m.Body(), a.name), "@")).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, m.Time().Format("15:04")).FontSize(10).TextColor(t.TextMuted.Alpha(0.7))
			})
			return
		}
		compact := a.settings.Compact
		size := avatarSize(compact)
		row := ui.Row(c).Gap(10).Margin(0, 8).Padding(3, 12).Radius(10).AlignItems(ui.Start)
		switch {
		case compact && !grouped:
			row.Padding(4, 12, 1)
		case compact:
			row.Padding(1, 12)
		case !grouped:
			row.Padding(7, 12, 3)
		}
		picking := a.pickerOpen && a.picking == m.TS
		hovered := row.Hovered() || picking
		if hovered {
			row.Background(t.Surface.Alpha(0.7))
		}
		row.Children(func() {
			if grouped {
				label := ""
				if hovered {
					label = m.Time().Format("15:04")
				}
				ui.Text(c, label).FontSize(10).TextColor(t.TextMuted).Width(size).TextAlign(ui.End).Padding(4, 0, 0)
			} else {
				avatar(c, a.author(m), a.picture(a.avatarURL(m)), size)
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				if !grouped {
					ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
						ui.Text(c, a.author(m)).FontWeight(600)
						ui.Text(c, m.Time().Format("15:04")).FontSize(11).TextColor(t.TextMuted)
					})
				}
				if md := a.markdown(m); md != "" {
					markdown.View(c, md, &a.md)
				}
				a.files(c, m)
				a.reactions(c, ch, m)
				if threads && m.ReplyCount > 0 {
					a.replies(c, ch, m)
				}
			})
			if hovered {
				a.actions(c, ch, m, threads)
			}
		})
	})
}

var (
	smileIcon = icon(`<circle cx="8" cy="8" r="6"/><path d="M5.6 9.6a3 3 0 0 0 4.8 0"/><path d="M6 6.2v.4M10 6.2v.4"/>`)
	replyIcon = icon(`<path d="M6.5 4 2.5 8l4 4"/><path d="M2.8 8h6.7a4 4 0 0 1 4 4v1"/>`)
)

// frequent are the reactions the picker offers before a search.
var frequent = []string{"+1", "heart", "joy", "tada", "eyes", "pray", "white_check_mark", "fire",
	"raised_hands", "clap", "100", "rocket", "thinking_face", "smile", "sob", "ok_hand",
	"muscle", "sparkles", "wave", "point_up", "heavy_plus_sign", "x", "warning", "bulb"}

// actions float over the top corner of the message under the pointer, as
// Slack's: react, and reply in a thread.
func (a *app) actions(c *ui.Context, ch *channel, m *slack.Message, threads bool) {
	ui.Row(c).Absolute().Top(-14).Right(14).Gap(2).Padding(3).Radius(12).Material(glass.Glass{}).
		Shadow(0, 2, 10, 0, ui.RGBA(0, 0, 0, 0.1)).Children(func() {
		react := a.actionButton(c.Key("react"), smileIcon, "添加表情")
		if react.Clicked() {
			a.picking, a.pickerOpen, a.pickerQuery = m.TS, true, ""
		}
		// One picker is open at most, on the message picking.
		open, closed := &a.pickerOpen, false
		if a.picking != m.TS {
			open = &closed
		}
		ui.Popover(c, react, open, func() { a.picker(c, ch, m) })
		if threads {
			if a.actionButton(c.Key("reply"), replyIcon, "在线程中回复").Clicked() {
				a.openThread(ch, m.TS)
			}
		}
	})
}

func (a *app) actionButton(c *ui.Context, svg *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(28, 26).Radius(9).Center().Label(label).Tooltip(label).Cursor(ui.CursorPointer).TextColor(t.TextMuted)
	if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.Children(func() { ui.Icon(c, svg).Size(15, 15) })
	return b
}

// picker chooses an emoji to react with, by search or among the frequent.
func (a *app) picker(c *ui.Context, ch *channel, m *slack.Message) {
	t := c.Theme()
	names := frequent
	if q := strings.TrimSpace(a.pickerQuery); q != "" {
		names = slack.SearchEmoji(q, 48)
	}
	ui.Column(c).Width(300).Gap(8).Padding(4).Children(func() {
		search := ui.TextInput(c, &a.pickerQuery).Placeholder("搜索表情").AutoFocus().Radius(8)
		if search.Submitted() && len(names) > 0 {
			a.toggleReaction(ch, m, names[0])
		}
		if len(names) == 0 {
			ui.Text(c, "没有找到表情").FontSize(12).TextColor(t.TextMuted).Padding(12).AlignSelf(ui.Center)
			return
		}
		ui.Grid(c).Columns(8).Gap(2).Children(func() {
			for _, n := range names {
				b := ui.ButtonBase(c.Key(n)).Size(34, 34).Radius(8).Center().Label(n).Tooltip(":" + n + ":").Cursor(ui.CursorPointer)
				if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.Children(func() { ui.Text(c, slack.Emoji(n)).FontSize(20) })
				if b.Clicked() {
					a.toggleReaction(ch, m, n)
				}
			}
		})
	})
}

// toggleReaction reacts with the emoji chosen, or takes the reaction
// back when the user had it, and closes the picker.
func (a *app) toggleReaction(ch *channel, m *slack.Message, name string) {
	mine := slices.ContainsFunc(m.Reactions, func(r slack.Reaction) bool {
		return r.Name == name && slices.Contains(r.Users, a.auth.UserID)
	})
	a.pickerOpen = false
	a.react(ch.ID, m.TS, name, !mine)
}

func avatarSize(compact bool) float32 {
	if compact {
		return 22
	}
	return 32
}

// notice reports a message Slack writes about the conversation, as a
// join or a new topic, rather than one a person wrote.
func notice(m *slack.Message) bool {
	switch m.Subtype {
	case "channel_join", "channel_leave", "group_join", "group_leave", "channel_topic",
		"channel_purpose", "channel_name", "group_topic", "group_purpose", "group_name",
		"bot_add", "bot_remove", "pinned_item", "unpinned_item", "channel_archive", "channel_unarchive":
		return true
	}
	return false
}

// replies opens a message's thread.
func (a *app) replies(c *ui.Context, ch *channel, m *slack.Message) {
	t := c.Theme()
	ui.Row(c).Children(func() {
		b := ui.ButtonBase(c).Padding(3, 8, 3, 4).Margin(0, 0, 0, -4).Radius(8).Cursor(ui.CursorPointer)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, bubbleIcon).Size(13, 13).TextColor(t.Accent)
				ui.Text(c, fmt.Sprintf("%d 条回复", m.ReplyCount)).FontSize(12).FontWeight(600).TextColor(t.Accent)
			})
		})
		if b.Clicked() {
			a.openThread(ch, m.TS)
		}
	})
}

// files shows a message's pictures as thumbnails, which open in Preview,
// and its other files as cards that download them.
func (a *app) files(c *ui.Context, m *slack.Message) {
	t := c.Theme()
	if len(m.Files) == 0 {
		return
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, f := range m.Files {
			if f.Thumb360 != "" && f.Thumb360W > 0 {
				w, h := float32(f.Thumb360W), float32(f.Thumb360H)
				if h > 240 {
					w, h = w*240/h, 240
				}
				b := ui.ButtonBase(c.Key("file:"+f.ID)).Size(w, h).Radius(12).Clip().Label(f.Name).Tooltip(f.Name).Cursor(ui.CursorPointer)
				b.Children(func() {
					if p := a.picture(f.Thumb360); p != nil {
						ui.Image(c, p).Size(w, h)
					} else {
						ui.Box(c).Size(w, h).Background(t.Surface)
					}
					ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(12).Border(1, t.Border.Alpha(0.5)).PassThrough()
					if d := a.downloads[f.ID]; d != nil && !d.finished && d.err == "" {
						ui.Box(c).Absolute().Left(10).Right(10).Bottom(10).Children(func() {
							ui.Progress(c, fraction(d))
						})
					}
				})
				if b.Clicked() {
					a.downloadFile(f, true)
				}
				continue
			}
			a.fileCard(c, f)
		}
	})
}

// fileCard is a file as Finder's list shows one: its kind, name and size,
// or how much of it has come. A click saves it to Downloads, or opens it
// once saved.
func (a *app) fileCard(c *ui.Context, f slack.File) {
	t := c.Theme()
	d := a.downloads[f.ID]
	name := f.Title
	if name == "" {
		name = f.Name
	}
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		card := ui.ButtonBase(c.Key("file:"+f.ID)).Width(300).Padding(8, 12, 8, 8).Radius(10).
			Background(t.Surface).Border(1, t.Border.Alpha(0.5)).Label(name).Cursor(ui.CursorPointer)
		if card.Hovered() {
			card.Background(t.SurfaceHover)
		}
		card.Children(func() {
			ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(32, 32).Radius(7).Center().Background(t.Accent.Alpha(0.12)).Children(func() {
					ui.Icon(c, docIcon).Size(18, 18).TextColor(t.Accent)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, name).FontWeight(500).SingleLine()
					switch {
					case d != nil && d.err != "":
						ui.Text(c, "下载失败，点按重试").FontSize(11).TextColor(t.Danger).Tooltip(d.err)
					case d != nil && !d.finished:
						ui.Progress(c, fraction(d))
					default:
						kind := f.Filetype
						if s := size(f.Size); s != "" {
							kind += " · " + s
						}
						if d != nil && d.finished {
							kind = "已下载 · 点按打开"
						}
						ui.Text(c, kind).FontSize(11).TextColor(t.TextMuted).SingleLine()
					}
				})
				if d == nil || d.err != "" {
					ui.Icon(c, downloadIcon).Size(16, 16).TextColor(t.TextMuted)
				}
			})
		})
		if card.Clicked() {
			if d != nil && d.err != "" {
				delete(a.downloads, f.ID)
			}
			a.downloadFile(f, false)
		}
		if d != nil && d.finished {
			if a.toolbarButton(c.Key("reveal:"+f.ID), revealIcon, "在 Finder 中显示").Clicked() {
				a.revealFile(d.path)
			}
		}
	})
}

func fraction(d *download) float64 {
	if d.total <= 0 {
		return 0
	}
	return float64(d.done) / float64(d.total)
}

var (
	docIcon      = icon(`<path d="M4 1.8h5.2L12.5 5v8.2a1 1 0 0 1-1 1h-7.5a1 1 0 0 1-1-1V2.8a1 1 0 0 1 1-1z"/><path d="M9 1.8V5h3.5"/>`)
	downloadIcon = icon(`<path d="M8 2.5v8M4.8 7.5 8 10.7l3.2-3.2M3 13.5h10"/>`)
	revealIcon   = icon(`<circle cx="7" cy="7" r="4.2"/><path d="M10.2 10.2 13.5 13.5"/>`)
)

// reactions shows a message's reactions as capsules; a click adds the
// user's or takes it back.
func (a *app) reactions(c *ui.Context, ch *channel, m *slack.Message) {
	t := c.Theme()
	if len(m.Reactions) == 0 {
		return
	}
	ui.Row(c).Wrap().Gap(6).Padding(2, 0).Children(func() {
		for _, r := range m.Reactions {
			mine := slices.Contains(r.Users, a.auth.UserID)
			label := slack.Emoji(r.Name)
			if label == "" {
				label = ":" + r.Name + ":"
			}
			chip := ui.ButtonBase(c).Height(24).Padding(0, 9).Radius(12).Cursor(ui.CursorPointer).Tooltip(a.reactors(r))
			switch {
			case mine:
				chip.Background(t.Accent.Alpha(0.14)).Border(1, t.Accent.Alpha(0.55))
			case chip.Hovered():
				chip.Background(t.SurfaceHover).Border(1, t.Border.Alpha(0.6))
			default:
				chip.Background(t.Surface).Border(1, t.Border.Alpha(0.6))
			}
			chip.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).FontSize(13)
					count := ui.Textf(c, "%d", r.Count).FontSize(11).FontWeight(600).TextColor(t.TextMuted)
					if mine {
						count.TextColor(t.Accent)
					}
				})
			})
			if chip.Clicked() {
				a.react(ch.ID, m.TS, r.Name, !mine)
			}
		}
	})
}

// composer is a capsule of glass: Enter sends, Shift+Enter breaks the
// line, and the arrow sends too.
func (a *app) composer(c *ui.Context, draft *string, placeholder string, send func(string)) {
	t := c.Theme()
	submit := func() {
		if text := strings.TrimSpace(*draft); text != "" {
			send(text)
			*draft = ""
		}
	}
	ui.Row(c).Padding(4, 4, 4, 14).Gap(8).Radius(18).AlignItems(ui.End).Material(glass.Glass{}).
		Shadow(0, 2, 12, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
		ui.TextAreaBase(c, draft).Placeholder(placeholder).Lines(1, 8).Grow(1).Padding(6, 0).OnTextKey(func(k ui.TextKey) bool {
			want := ui.Modifiers(0)
			if a.settings.CmdEnter {
				want = ui.Super
			}
			if k.Key != ui.KeyEnter || k.Mods != want {
				return false
			}
			if text := strings.TrimSpace(k.Text()); text != "" {
				send(text)
				k.SetText("")
			}
			return true
		})
		ready := strings.TrimSpace(*draft) != ""
		b := ui.ButtonBase(c).Size(28, 28).Radius(14).Center().Label("发送").Disabled(!ready)
		if ready {
			b.Material(glass.Glass{Tint: t.Accent, Interactive: true}).TextColor(t.AccentText).Cursor(ui.CursorPointer)
		} else {
			b.Background(t.Surface).TextColor(t.TextMuted)
		}
		b.Children(func() { ui.Icon(c, sendIcon).Size(15, 15) })
		if b.Clicked() {
			submit()
		}
	})
}
