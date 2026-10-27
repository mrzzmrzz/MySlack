package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans" // MyGo's own texts in Chinese
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/mrzzmrzz/MySlack/slack"

	"github.com/egoist/mygo/i18n"
	"github.com/mrzzmrzz/MySlack/locales"
)

// channel is a conversation with the messages loaded for it.
type channel struct {
	slack.Channel
	messages []slack.Message
	loaded   bool
	loading  bool
	err      string
	unread   bool
	list     ui.ListState
}

// thread is the thread open beside the channel.
type thread struct {
	channel  string
	ts       string
	messages []slack.Message
	loading  bool
	err      string
	list     ui.ListState
	draft    string
}

// app is the state the window shows. It changes on the main thread only:
// goroutines publish their results through win.Update.
type app struct {
	win    *mygo.Window
	client *slack.Client
	auth   *slack.Auth
	cancel context.CancelFunc

	// Sign-in form.
	tokenIn, cookieIn string
	signingIn         bool
	signInErr         string

	channels     map[string]*channel
	users        map[string]*slack.User
	pending      map[string]bool // users being looked up
	selected     string
	channelsOpen bool
	dmsOpen      bool
	draft        string
	thread       *thread
	connected    bool
	connErr      string

	md         markdown.Options
	services   ui.Services
	images     map[string]*picture  // avatars and thumbnails by URL
	downloads  map[string]*download // files saved, by ID
	cacheDir   string               // see cache.go
	cacheBytes int64                // the cache's size as the settings last saw it, -1 unknown
	measuring  bool
	fetching   map[string]bool // files fetched to preview, by ID
	previewing *pendingPreview // the preview waiting for its files

	sidebarHidden bool
	titleInset    float32 // of the toolbar's title, past the window's controls

	settings    settings
	settingsWin *mygo.Window
	settingsTab int

	// The profile shown, under the avatar of the message at profileAt.
	profileAt   string
	profileOpen bool

	// The emoji picker, open on the message picking.
	picking     string
	pickerOpen  bool
	pickerQuery string
}

// picture is an image of the workspace, loaded once.
type picture struct {
	bitmap *ui.Bitmap
	failed bool
}

func newApp() *app {
	a := &app{
		channels:     map[string]*channel{},
		users:        map[string]*slack.User{},
		pending:      map[string]bool{},
		channelsOpen: true,
		dmsOpen:      true,
		images:       map[string]*picture{},
		downloads:    map[string]*download{},
		fetching:     map[string]bool{},
		cacheDir:     defaultCacheDir(),
		cacheBytes:   -1,
		settings:     defaultSettings(),
	}
	a.md = markdown.Options{
		Theme:    &markdown.Theme{LineHeight: 1.45, BlockSpacing: 0.5},
		WrapCode: true,
		OnLink:   a.openLink,
	}
	return a
}

// update publishes a goroutine's result on the main thread; without a
// window, as in tests, it applies it at once, one goroutine at a time, as
// the main thread would.
func (a *app) update(fn func()) {
	if a.win == nil {
		updates.Lock()
		defer updates.Unlock()
		fn()
		return
	}
	a.win.Update(fn)
}

var updates sync.Mutex

// Sign-in

const secretKey = "session"

type session struct {
	Token  string `json:"token"`
	Cookie string `json:"cookie"`
}

// signIn checks the credentials, then loads the workspace and listens to
// it. save keeps credentials that work in the Keychain.
func (a *app) signIn(token, cookie string, save bool) {
	a.signingIn, a.signInErr = true, ""
	client := slack.New(token, cookie)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		auth, err := client.AuthTest(ctx)
		if err == nil && save {
			data, _ := json.Marshal(session{client.Token, client.Cookie})
			if serr := mygo.Secrets.Set(secretKey, data); serr != nil {
				log.Printf("keychain: %v", serr)
			}
		}
		a.update(func() {
			a.signingIn = false
			if err != nil {
				a.signInErr = err.Error()
				return
			}
			a.client, a.auth = client, auth
			a.tokenIn, a.cookieIn = "", ""
			a.resolve([]string{auth.UserID})
			a.start()
		})
	}()
}

func (a *app) signOut() {
	if a.cancel != nil {
		a.cancel()
	}
	if err := mygo.Secrets.Delete(secretKey); err != nil && !errors.Is(err, mygo.ErrSecretNotFound) {
		log.Printf("keychain: %v", err)
	}
	win, s, sw := a.win, a.settings, a.settingsWin
	*a = *newApp()
	a.md.OnLink = a.openLink
	a.win, a.settings, a.settingsWin = win, s, sw
	a.updateBadge()
}

// start loads the conversations and opens the real-time socket.
func (a *app) start() {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	client := a.client
	go func() {
		chs, err := client.Conversations(ctx)
		unreads, _ := client.Unreads(ctx)
		a.update(func() {
			if err != nil {
				a.connErr = err.Error()
				return
			}
			var users []string
			for _, ch := range chs {
				a.channels[ch.ID] = &channel{Channel: ch, unread: unreads[ch.ID]}
				if ch.IsIM {
					users = append(users, ch.User)
				}
			}
			a.resolve(users)
		})
	}()
	go client.Listen(ctx, func(e slack.Event) {
		a.update(func() { a.handle(e) })
	}, func(ok bool, err error) {
		a.update(func() {
			a.connected = ok
			a.connErr = ""
			if err != nil {
				a.connErr = err.Error()
			}
		})
	})
}

// Names

// resolve looks up the users not known yet.
func (a *app) resolve(ids []string) {
	var missing []string
	for _, id := range ids {
		if id == "" || a.users[id] != nil || a.pending[id] {
			continue
		}
		a.pending[id] = true
		missing = append(missing, id)
	}
	if len(missing) == 0 || a.client == nil {
		return
	}
	client := a.client
	go func() {
		for _, id := range missing {
			u, err := client.UserInfo(context.Background(), id)
			a.update(func() {
				delete(a.pending, id)
				if err == nil {
					a.users[id] = u
				}
			})
		}
	}()
}

var mention = regexp.MustCompile(`<@([UW][A-Z0-9]+)`)

// userIDs collects the authors and mentioned users of messages.
func userIDs(ms []slack.Message) []string {
	var ids []string
	for _, m := range ms {
		ids = append(ids, m.User)
		for _, match := range mention.FindAllStringSubmatch(m.Text, -1) {
			ids = append(ids, match[1])
		}
	}
	return ids
}

func (a *app) name(id string) string {
	if u := a.users[id]; u != nil {
		return u.Label()
	}
	if ch := a.channels[id]; ch != nil {
		return ch.Name
	}
	return ""
}

func (a *app) authorName(m *slack.Message) string {
	switch {
	case m.User != "":
		if n := a.name(m.User); n != "" {
			return n
		}
		return m.User
	case m.Username != "":
		return m.Username
	case m.BotID != "":
		return "bot"
	}
	return "Slack"
}

func (a *app) title(ch *channel) string {
	switch {
	case ch.IsIM:
		if n := a.name(ch.User); n != "" {
			return n
		}
		return ch.User
	case ch.IsMPIM:
		// mpdm-alice--bob--carol-1
		s := strings.TrimPrefix(ch.Name, "mpdm-")
		if i := strings.LastIndex(s, "-"); i > 0 {
			s = s[:i]
		}
		return strings.ReplaceAll(s, "--", ", ")
	case ch.IsPrivate:
		return "🔒 " + ch.Name
	}
	return "# " + ch.Name
}

// Sidebar

// open shows a channel, loading its history the first time.
func (a *app) open(id string) {
	ch := a.channels[id]
	if ch == nil {
		return
	}
	a.thread = nil
	ch.unread = false
	a.updateBadge()
	if a.win != nil {
		a.win.SetTitle(a.title(ch))
	}
	if ch.loaded {
		a.markRead(ch)
		return
	}
	a.load(ch)
}

func (a *app) load(ch *channel) {
	if ch.loading || a.client == nil {
		return
	}
	ch.loading, ch.err = true, ""
	client := a.client
	go func() {
		ms, err := client.History(context.Background(), ch.ID, 100)
		a.update(func() {
			ch.loading = false
			if err != nil {
				ch.err = err.Error()
				return
			}
			ch.messages, ch.loaded = ms, true
			a.resolve(userIDs(ms))
			if a.selected == ch.ID {
				a.markRead(ch)
			}
		})
	}()
}

func (a *app) markRead(ch *channel) {
	if len(ch.messages) == 0 || a.client == nil {
		return
	}
	client, ts := a.client, ch.messages[len(ch.messages)-1].TS
	go func() {
		if err := client.Mark(context.Background(), ch.ID, ts); err != nil {
			log.Printf("mark %s: %v", ch.ID, err)
		}
	}()
}

// Messages

// markdown is the text of a message and its attachments as Markdown.
func (a *app) markdown(m *slack.Message) string {
	parts := []string{}
	if m.Text != "" {
		parts = append(parts, slack.Markdown(m.Text, a.name))
	}
	for _, at := range m.Attachments {
		var lines []string
		for _, s := range []string{at.Pretext, at.Title, at.Text} {
			if s != "" && s != m.Text {
				lines = append(lines, slack.Markdown(s, a.name))
			}
		}
		if len(lines) == 0 && at.Fallback != "" && at.Fallback != m.Text {
			lines = append(lines, slack.Markdown(at.Fallback, a.name))
		}
		if len(lines) > 0 {
			parts = append(parts, "> "+strings.ReplaceAll(strings.Join(lines, "\n\n"), "\n", "\n> "))
		}
	}
	md := strings.Join(parts, "\n\n")
	return md
}

func (a *app) reactors(r slack.Reaction) string {
	names := make([]string, 0, len(r.Users))
	for _, id := range r.Users {
		if n := a.name(id); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, "、")
}

// react adds the user's reaction or takes it back, showing it at once.
func (a *app) react(channelID, ts, name string, add bool) {
	a.reacted(channelID, ts, name, a.auth.UserID, add)
	client := a.client
	go func() {
		if err := client.React(context.Background(), channelID, ts, name, add); err != nil {
			log.Printf("react: %v", err)
		}
	}()
}

func (a *app) avatarURL(m *slack.Message) string {
	if u := a.users[m.User]; u != nil {
		return u.Photo()
	}
	return ""
}

// picture returns the image at url, starting to load it the first time
// it is asked for, from the cache or from Slack; nil until it has loaded.
func (a *app) picture(url string) *ui.Bitmap {
	if url == "" || a.client == nil {
		return nil
	}
	if p := a.images[url]; p != nil {
		return p.bitmap
	}
	p := &picture{}
	a.images[url] = p
	client, path := a.client, a.imagePath(url)
	go func() {
		// From the cache, else from Slack into the cache.
		data, ok := cached(path)
		var err error
		if !ok {
			if data, err = client.Fetch(context.Background(), url); err == nil {
				keep(path, data)
			}
		}
		var bmp *ui.Bitmap
		if err == nil {
			bmp, err = ui.DecodeBitmap(data)
		}
		a.update(func() {
			p.bitmap, p.failed = bmp, err != nil
		})
	}()
	return nil
}

// openLink opens links of messages: Slack's own in the app, the others
// in the browser.
func (a *app) openLink(url string) {
	switch {
	case strings.HasPrefix(url, "slack://channel/"):
		id := strings.TrimPrefix(url, "slack://channel/")
		if a.channels[id] != nil {
			a.selected = id
			a.open(id)
		}
	case strings.HasPrefix(url, "slack://user/"):
		id := strings.TrimPrefix(url, "slack://user/")
		for _, ch := range a.channels {
			if ch.IsIM && ch.User == id {
				a.selected = ch.ID
				a.open(ch.ID)
				return
			}
		}
	case ui.SafeToOpen(url):
		a.services.OpenURL(url)
	}
}

func sameDay(x, y time.Time) bool {
	return x.Year() == y.Year() && x.YearDay() == y.YearDay()
}

func day(tm time.Time) string {
	now := time.Now()
	switch {
	case sameDay(tm, now):
		return locales.Today()
	case sameDay(tm, now.AddDate(0, 0, -1)):
		return locales.Yesterday()
	case tm.Year() == now.Year():
		return date(tm, "MMMEEEEd") // Monday, January 2; 1月2日星期一
	}
	return date(tm, "yMMMd")
}

// date is tm as the app's language writes the fields of a CLDR skeleton.
func date(tm time.Time, skeleton string) string {
	return i18n.FormatDate(i18n.Locale(), tm, i18n.DateOptions{Skeleton: skeleton})
}

func stamp(tm time.Time) string {
	now := time.Now()
	if y, m, d := tm.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		return tm.Format("15:04")
	}
	if tm.Year() == now.Year() {
		return date(tm, "MMMd") + " " + tm.Format("15:04")
	}
	return date(tm, "yMMMd") + " " + tm.Format("15:04")
}

func (a *app) send(channelID, text, threadTS string) {
	if a.client == nil {
		return
	}
	client := a.client
	go func() {
		err := client.Post(context.Background(), channelID, text, threadTS)
		a.update(func() {
			if err != nil {
				if ch := a.channels[channelID]; ch != nil {
					ch.err = locales.SendFailed(err.Error())
				}
				return
			}
			// The socket echoes what was sent; without it, load again.
			if !a.connected {
				if ch := a.channels[channelID]; ch != nil {
					ch.loaded = false
					a.load(ch)
				}
			}
		})
	}()
}

func (a *app) openThread(ch *channel, ts string) {
	th := &thread{channel: ch.ID, ts: ts, loading: true}
	a.thread = th
	client := a.client
	go func() {
		ms, err := client.Replies(context.Background(), ch.ID, ts)
		a.update(func() {
			th.loading = false
			if err != nil {
				th.err = err.Error()
				return
			}
			th.messages = ms
			a.resolve(userIDs(ms))
		})
	}()
}

// Real-time events

func (a *app) handle(e slack.Event) {
	if e.Type == "reaction_added" || e.Type == "reaction_removed" {
		a.reacted(e.Item.Channel, e.Item.TS, e.Reaction, e.User, e.Type == "reaction_added")
		return
	}
	ch := a.channels[e.Channel]
	if ch == nil {
		return
	}
	th := a.thread
	if th != nil && th.channel != ch.ID {
		th = nil
	}
	switch e.Subtype {
	case "message_changed", "message_replied":
		if e.Changed == nil {
			return
		}
		replace(ch.messages, *e.Changed)
		if th != nil {
			replace(th.messages, *e.Changed)
		}
		a.resolve(userIDs([]slack.Message{*e.Changed}))
		return
	case "message_deleted":
		ch.messages = remove(ch.messages, e.DeletedTS)
		if th != nil {
			th.messages = remove(th.messages, e.DeletedTS)
		}
		return
	}
	m := e.Message
	a.resolve(userIDs([]slack.Message{m}))
	if th != nil && (m.ThreadTS == th.ts) {
		th.messages = appendNew(th.messages, m)
	}
	if m.IsReply() {
		for i := range ch.messages {
			if ch.messages[i].TS == m.ThreadTS {
				ch.messages[i].ReplyCount++
			}
		}
		return
	}
	if ch.loaded {
		ch.messages = appendNew(ch.messages, m)
	}
	if m.User == a.auth.UserID {
		return
	}
	focused := a.win != nil && a.win.IsFocused()
	if a.selected == ch.ID {
		a.markRead(ch)
	} else {
		ch.unread = true
		a.updateBadge()
	}
	if a.selected != ch.ID || !focused {
		a.notify(ch, &m)
	}
}

// notify shows a desktop notification of a message, as the settings say.
func (a *app) notify(ch *channel, m *slack.Message) {
	s := a.settings
	if !s.Notify || a.win == nil {
		return
	}
	mentioned := strings.Contains(m.Text, "<@"+a.auth.UserID+">")
	if s.OnlyMention && !ch.IsIM && !mentioned {
		return
	}
	subtitle := a.title(ch)
	if ch.IsIM {
		subtitle = ""
	}
	n := mygo.NewNotification(mygo.NotificationOptions{
		Title:    a.authorName(m),
		Subtitle: subtitle,
		Body:     slack.PlainText(m.Body(), a.name),
		Group:    ch.ID,
		Silent:   !s.Sound,
	})
	n.OnClick(func() {
		a.win.Update(func() {
			a.selected = ch.ID
			a.open(ch.ID)
		})
		a.win.Show()
		a.win.Focus()
	})
	if err := n.Show(); err != nil {
		log.Printf("notification: %v", err)
	}
}

// updateBadge shows how many conversations have unread messages on the
// Dock's icon.
func (a *app) updateBadge() {
	if a.win == nil {
		return
	}
	n := 0
	if a.settings.DockBadge {
		for _, ch := range a.channels {
			if ch.unread {
				n++
			}
		}
	}
	mygo.App.SetBadgeCount(n)
}

func replace(ms []slack.Message, m slack.Message) {
	for i := range ms {
		if ms[i].TS == m.TS {
			ms[i] = m
		}
	}
}

func remove(ms []slack.Message, ts string) []slack.Message {
	return slices.DeleteFunc(ms, func(m slack.Message) bool { return m.TS == ts })
}

func appendNew(ms []slack.Message, m slack.Message) []slack.Message {
	if slices.ContainsFunc(ms, func(x slack.Message) bool { return x.TS == m.TS }) {
		return ms
	}
	return append(ms, m)
}

func main() {
	a := newApp()
	// The app shows the language of its catalogs (locales/) the user reads
	// best, as the system lists their languages, and follows a change of
	// them: the views say their messages anew every frame, and what was set
	// once is set again when it changes.
	mygo.App.WhenReady(func() {
		migrateIdentifier()
		a.settings = loadSettings()
		mygo.App.SetMenu(a.menu())
		mygo.App.OnLocaleChanged(func(string) {
			mygo.App.SetMenu(a.menu())
			if a.settingsWin != nil {
				a.settingsWin.SetTitle(locales.SettingsTitle())
			}
		})
		// A made-up workspace for screenshots, or the session kept from
		// last time.
		demo, isDemo := os.LookupEnv("SLACK_MYGO_DEMO")
		stateKey := "main"
		if isDemo {
			a.settings = defaultSettings()
			a.loadDemo(demo)
			stateKey = "" // the window's default size
		} else if data, err := mygo.Secrets.Get(secretKey); err == nil {
			var s session
			if json.Unmarshal(data, &s) == nil && s.Token != "" {
				defer a.signIn(s.Token, s.Cookie, false)
			}
		} else if !errors.Is(err, mygo.ErrSecretNotFound) {
			log.Printf("keychain: %v", err)
		}
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Slack",
			Width:     1100,
			Height:    720,
			MinWidth:  720,
			MinHeight: 460,
			StateKey:  stateKey,
			// The traffic lights sit over the sidebar, whose material is
			// the system's.
			TitleBarStyle:  mygo.TitleBarHiddenInset,
			TitleBarHeight: titleBarHeight,
			Vibrancy:       mygo.VibrancySidebar,
			Content:        ui.View(a.view),
		})
		a.keepRunning()
		go a.trimCache(cacheLimit)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// reacted applies a reaction added or removed by user to its message.
func (a *app) reacted(channelID, ts, name, user string, add bool) {
	ch := a.channels[channelID]
	if ch == nil {
		return
	}
	lists := [][]slack.Message{ch.messages}
	if a.thread != nil && a.thread.channel == ch.ID {
		lists = append(lists, a.thread.messages)
	}
	for _, ms := range lists {
		for i := range ms {
			if ms[i].TS == ts {
				ms[i].Reactions = applyReaction(ms[i].Reactions, name, user, add)
			}
		}
	}
	a.resolve([]string{user})
}

func applyReaction(rs []slack.Reaction, name, user string, add bool) []slack.Reaction {
	i := slices.IndexFunc(rs, func(r slack.Reaction) bool { return r.Name == name })
	if add {
		if i < 0 {
			return append(rs, slack.Reaction{Name: name, Count: 1, Users: []string{user}})
		}
		if !slices.Contains(rs[i].Users, user) {
			rs[i].Count++
			rs[i].Users = append(rs[i].Users, user)
		}
		return rs
	}
	if i < 0 || !slices.Contains(rs[i].Users, user) {
		return rs
	}
	rs[i].Count--
	rs[i].Users = slices.DeleteFunc(rs[i].Users, func(u string) bool { return u == user })
	if rs[i].Count <= 0 {
		return slices.Delete(rs, i, i+1)
	}
	return rs
}
