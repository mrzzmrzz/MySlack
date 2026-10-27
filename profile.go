package main

import (
	"context"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/mrzzmrzz/Slack-MyGo/slack"
)

// person is a message's avatar, or its author's name with isAvatar false,
// as a button showing the author's profile in a popover under the avatar,
// as Slack's do. key tells apart the same message in the channel and in
// its thread.
func (a *app) person(c *ui.Context, m *slack.Message, key string, size float32, isAvatar bool) {
	open := a.profileOpen && a.profileAt == key
	if !isAvatar {
		name := ui.ButtonBase(c.Key("name:" + key)).Label(a.authorName(m)).Cursor(ui.CursorPointer)
		name.Children(func() {
			n := ui.Text(c, a.authorName(m)).FontWeight(600)
			if name.Hovered() || open {
				n.Underline()
			}
		})
		if name.Clicked() && m.User != "" {
			a.profileAt, a.profileOpen = key, true
		}
		return
	}
	b := ui.ButtonBase(c.Key("avatar:" + key)).Radius(size / 2).Label(a.authorName(m)).Cursor(ui.CursorPointer)
	b.Children(func() { avatar(c, a.authorName(m), a.picture(a.avatarURL(m)), size) })
	if b.Hovered() || open {
		b.Opacity(0.85)
	}
	if b.Clicked() && m.User != "" {
		a.profileAt, a.profileOpen = key, true
	}
	shown, closed := &a.profileOpen, false
	if a.profileAt != key {
		shown = &closed
	}
	ui.Popover(c, b, shown, func() { a.profile(c, m.User) })
}

// profile is a person's card: picture, name, title, status and the time
// where they are, and a button to write to them.
func (a *app) profile(c *ui.Context, id string) {
	t := c.Theme()
	u := a.users[id]
	name, title, photo := id, "", (*ui.Bitmap)(nil)
	if u != nil {
		name, title, photo = u.Label(), u.Profile.Title, a.picture(u.LargePhoto())
	}
	ui.Column(c).Width(260).Padding(12, 8, 8).Gap(10).AlignItems(ui.Center).Children(func() {
		avatar(c, name, photo, 72)
		ui.Column(c).Gap(2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, name).FontSize(16).FontWeight(600).TextAlign(ui.Center)
			if title != "" {
				ui.Text(c, title).FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center)
			}
		})
		if u != nil && (u.Profile.StatusText != "" || u.Profile.StatusEmoji != "") {
			ui.Row(c).Gap(6).Padding(4, 10).Radius(10).Background(t.Surface).AlignItems(ui.Center).Children(func() {
				if e := slack.Emoji(trimColons(u.Profile.StatusEmoji)); e != "" {
					ui.Text(c, e)
				}
				if u.Profile.StatusText != "" {
					ui.Text(c, u.Profile.StatusText).FontSize(12)
				}
			})
		}
		if u != nil && u.TZ != "" {
			if loc, err := time.LoadLocation(u.TZ); err == nil {
				ui.Text(c, "当地时间 "+time.Now().In(loc).Format("15:04")).FontSize(12).TextColor(t.TextMuted)
			}
		}
		if id != a.auth.UserID && (u == nil || !u.IsBot) {
			b := ui.ButtonBase(c.Key("write")).AlignSelf(ui.Stretch).Height(30).Radius(15).Center().Label("发消息").
				Material(glass.Glass{Tint: t.Accent, Interactive: true}).Cursor(ui.CursorPointer)
			b.Children(func() { ui.Text(c, "发消息").FontWeight(600).TextColor(t.AccentText) })
			if b.Clicked() {
				a.profileOpen = false
				a.openDM(id)
			}
		}
	})
}

// openDM shows the direct conversation with a person, starting it on
// Slack when there is none.
func (a *app) openDM(user string) {
	for _, ch := range a.channels {
		if ch.IsIM && ch.User == user {
			a.selected = ch.ID
			a.open(ch.ID)
			return
		}
	}
	if a.client == nil {
		return
	}
	client := a.client
	go func() {
		ch, err := client.OpenIM(context.Background(), user)
		a.update(func() {
			if err != nil {
				a.connErr = err.Error()
				return
			}
			if a.channels[ch.ID] == nil {
				a.channels[ch.ID] = &channel{Channel: *ch}
			}
			a.selected = ch.ID
			a.open(ch.ID)
		})
	}()
}

func trimColons(s string) string {
	if len(s) > 1 && s[0] == ':' && s[len(s)-1] == ':' {
		return s[1 : len(s)-1]
	}
	return s
}
