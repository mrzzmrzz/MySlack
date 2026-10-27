package main

import (
	"slices"

	"github.com/egoist/mygo"
)

// menu is the menu bar, with the items and shortcuts a Mac app has: the
// app's, File's ⌘W, Edit's, View's, Go's moves between conversations, and
// Window's.
func (a *app) menu() *mygo.Menu {
	do := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			if a.win != nil {
				a.win.Update(fn)
			}
		}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Slack-MyGo", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			mygo.Separator(),
			{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings() }},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "显示", Submenu: []*mygo.MenuItem{
			{Label: "显示或隐藏边栏", Accelerator: "Ctrl+Cmd+S", Click: do(func() { a.sidebarHidden = !a.sidebarHidden })},
			mygo.Separator(),
			{Label: "放大", Accelerator: "CmdOrCtrl+Plus", Click: do(func() { a.zoom(1) })},
			{Label: "缩小", Accelerator: "CmdOrCtrl+-", Click: do(func() { a.zoom(-1) })},
			{Label: "实际大小", Accelerator: "CmdOrCtrl+0", Click: do(func() { a.zoom(0) })},
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: "前往", Submenu: []*mygo.MenuItem{
			{Label: "上一个会话", Accelerator: "Alt+Up", Click: do(func() { a.step(-1, false) })},
			{Label: "下一个会话", Accelerator: "Alt+Down", Click: do(func() { a.step(1, false) })},
			{Label: "下一个未读会话", Accelerator: "Alt+Shift+Down", Click: do(func() { a.step(1, true) })},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

// step moves to the conversation n places from the one shown, in the
// sidebar's order, the next unread one with unread set; it wraps around.
func (a *app) step(n int, unread bool) {
	rooms, dms := a.conversations()
	order := append(rooms, dms...)
	if len(order) == 0 {
		return
	}
	i := slices.IndexFunc(order, func(ch *channel) bool { return ch.ID == a.selected })
	for range order {
		i = (i + n + len(order)) % len(order)
		if !unread || order[i].unread {
			a.selected = order[i].ID
			a.open(a.selected)
			return
		}
	}
}

// zoom makes the text larger or smaller, as ⌘+ and ⌘− do, or the standard
// size with 0.
func (a *app) zoom(n int) {
	size := 1
	if n != 0 {
		size = min(max(a.settings.TextSize+n, 0), len(textSizes)-1)
	}
	if size != a.settings.TextSize {
		a.settings.TextSize = size
		a.saveSettings()
	}
}

// keepRunning makes the main window hide when it is closed, rather than go,
// as Messages and Mail do: the app stays connected and notifies, and a
// click on its Dock icon shows the window again as it was.
func (a *app) keepRunning() {
	quitting := false
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if !quitting {
			e.PreventDefault()
			a.win.Hide()
		}
	})
	mygo.App.OnWindowAllClosed(func() {}) // keep running without windows
	// The system may count a window of its own as visible, so ask the
	// main window rather than trust hasVisibleWindows.
	mygo.App.OnActivate(func(bool) {
		if !a.win.IsVisible() {
			a.win.Show()
			a.win.Focus()
		}
	})
}
