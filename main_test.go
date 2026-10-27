package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/mrzzmrzz/Slack-MyGo/slack"
)

func TestSignInView(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 900, 600)
	if !tt.HasText("登录 Slack") {
		t.Errorf("texts %q", tt.Texts())
	}
}

// A signed-in app without a client shows its channels, a channel's
// messages, and the events the socket brings.
func TestChannels(t *testing.T) {
	a := newApp()
	a.auth = &slack.Auth{Team: "Acme", User: "me", UserID: "U0"}
	a.users["U1"] = &slack.User{ID: "U1", Name: "ada"}
	a.channels["C1"] = &channel{Channel: slack.Channel{ID: "C1", Name: "general", IsChannel: true}, loaded: true,
		messages: []slack.Message{{User: "U1", Text: "hi <@U1>", TS: "1700000000.000100", ReplyCount: 2}}}
	a.channels["C2"] = &channel{Channel: slack.Channel{ID: "C2", Name: "random", IsChannel: true}, loaded: true}
	a.channels["D1"] = &channel{Channel: slack.Channel{ID: "D1", IsIM: true, User: "U1"}, loaded: true}
	tt := ui.NewTester(a.view, 1000, 700)
	for _, s := range []string{"general", "random", "ada"} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click("general"); err != nil {
		t.Fatal(err)
	}
	if a.selected != "C1" || !tt.HasText("hi @ada") || !tt.HasText("2 条回复") {
		t.Errorf("selected %q, texts %q", a.selected, tt.Texts())
	}

	a.handle(slack.Event{Channel: "C1", Message: slack.Message{User: "U1", Text: "new one", TS: "1700000001.000100"}})
	a.handle(slack.Event{Channel: "C2", Message: slack.Message{User: "U1", Text: "elsewhere", TS: "1700000002.000100"}})
	a.handle(slack.Event{Channel: "C1", Message: slack.Message{User: "U1", Text: "reply", TS: "1700000003.000100", ThreadTS: "1700000000.000100"}})
	tt.Frame()
	if !tt.HasText("new one") || tt.HasText("reply") || !tt.HasText("3 条回复") {
		t.Errorf("texts %q", tt.Texts())
	}
	if !a.channels["C2"].unread || a.channels["C1"].unread {
		t.Error("unread marks")
	}
}

// A reaction chosen in the picker shows at once, and choosing it again
// takes it back.
func TestReactFromPicker(t *testing.T) {
	a := newApp()
	a.auth = &slack.Auth{UserID: "U0"}
	ch := &channel{Channel: slack.Channel{ID: "C1", Name: "general"}, loaded: true,
		messages: []slack.Message{{User: "U1", Text: "hi", TS: "1.1"}}}
	a.channels["C1"] = ch
	a.selected = "C1"
	a.client = slack.New("xoxc-test", "d")
	tt := ui.NewTester(a.view, 1000, 700)
	a.picking, a.pickerOpen = "1.1", true
	tt.Frame()
	tt.Frame()
	if err := tt.Click("tada"); err != nil {
		t.Fatalf("%v; texts %q", err, tt.Texts())
	}
	if rs := ch.messages[0].Reactions; len(rs) != 1 || rs[0].Name != "tada" || rs[0].Count != 1 || a.pickerOpen {
		t.Fatalf("reactions %+v, picker open %v", rs, a.pickerOpen)
	}
	a.toggleReaction(ch, &ch.messages[0], "tada")
	if len(ch.messages[0].Reactions) != 0 {
		t.Errorf("reactions %+v after taking it back", ch.messages[0].Reactions)
	}
}

// The sidebar's button hides and shows it, Escape closes a thread, and
// notices of joins read as a line without an author.
func TestChrome(t *testing.T) {
	a := newApp()
	a.auth = &slack.Auth{UserID: "U0"}
	a.users["U1"] = &slack.User{ID: "U1", Name: "ada"}
	a.channels["C1"] = &channel{Channel: slack.Channel{ID: "C1", Name: "general"}, loaded: true,
		messages: []slack.Message{
			{User: "U1", Subtype: "channel_join", Text: "<@U1> 已加入此频道", TS: "1.1"},
			{User: "U1", Text: "hi", TS: "2.1", ReplyCount: 1},
		}}
	a.selected = "C1"
	tt := ui.NewTester(a.view, 1000, 700)
	if !tt.HasText("ada 已加入此频道") {
		t.Errorf("texts %q", tt.Texts())
	}
	if err := tt.Click("隐藏边栏"); err != nil {
		t.Fatal(err)
	}
	if !a.sidebarHidden {
		t.Error("the sidebar shows after its button")
	}
	a.thread = &thread{channel: "C1", ts: "2.1"}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if a.thread != nil {
		t.Error("Escape left the thread open")
	}
}

// An author's profile shows their title, and its button opens the direct
// conversation with them.
func TestProfile(t *testing.T) {
	a := newApp()
	a.auth = &slack.Auth{UserID: "U0"}
	ada := &slack.User{ID: "U1", Name: "ada", TZ: "Europe/London"}
	ada.Profile.Title = "Analyst"
	a.users["U1"] = ada
	a.channels["C1"] = &channel{Channel: slack.Channel{ID: "C1", Name: "general"}, loaded: true,
		messages: []slack.Message{{User: "U1", Text: "hi", TS: "1.1"}}}
	a.channels["D1"] = &channel{Channel: slack.Channel{ID: "D1", IsIM: true, User: "U1"}, loaded: true}
	a.selected = "C1"
	tt := ui.NewTester(a.view, 1000, 700)
	// The sidebar has an "ada" too: open the message's profile as its
	// avatar's click does.
	a.profileAt, a.profileOpen = "true1.1", true
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Analyst") || !tt.HasText("发消息") {
		t.Fatalf("no profile in %q", tt.Texts())
	}
	if err := tt.Click("发消息"); err != nil {
		t.Fatal(err)
	}
	if a.selected != "D1" || a.profileOpen {
		t.Errorf("selected %q, profile open %v", a.selected, a.profileOpen)
	}
}

// ⌥↑ and ⌥↓ move through the conversations in the sidebar's order,
// wrapping around; ⌥⇧↓ skips to the next unread one.
func TestStep(t *testing.T) {
	a := newApp()
	a.auth = &slack.Auth{UserID: "U0"}
	for _, id := range []string{"C1", "C2", "C3"} {
		a.channels[id] = &channel{Channel: slack.Channel{ID: id, Name: strings.ToLower(id)}, loaded: true}
	}
	a.selected = "C1"
	a.step(1, false)
	if a.selected != "C2" {
		t.Errorf("down from c1: %q", a.selected)
	}
	a.step(-1, false)
	a.step(-1, false)
	if a.selected != "C3" {
		t.Errorf("up twice from c2, wrapping: %q", a.selected)
	}
	a.selected, a.channels["C3"].unread = "C1", true
	a.step(1, true)
	if a.selected != "C3" || a.channels["C3"].unread {
		t.Errorf("next unread from c1: %q", a.selected)
	}
}

// A click on a picture fetches it, and the others of its message, to show
// them in Quick Look; the thumbnail shows a spinner meanwhile.
func TestPreview(t *testing.T) {
	// A server that holds each request until the test ends, as a slow
	// download does.
	hold := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-hold
		w.WriteHeader(http.StatusServiceUnavailable) // and so writes no file
	}))
	defer srv.Close()
	defer close(hold)
	a := newApp()
	a.cacheDir = t.TempDir()
	a.auth = &slack.Auth{UserID: "U0"}
	a.client = slack.New("xoxc-test", "d")
	pic := func(id string) slack.File {
		return slack.File{ID: id, Name: id + ".png", Thumb360: "thumb", Thumb360W: 360, Thumb360H: 200, URLPrivateDownload: srv.URL + "/" + id}
	}
	a.channels["C1"] = &channel{Channel: slack.Channel{ID: "C1", Name: "general"}, loaded: true,
		messages: []slack.Message{{User: "U0", Text: "look", TS: "1.1", Files: []slack.File{pic("F1"), pic("F2")}}}}
	a.selected = "C1"
	tt := ui.NewTester(a.view, 1000, 700)
	if err := tt.Click("F2.png"); err != nil {
		t.Fatal(err)
	}
	if !a.fetching["F1"] || !a.fetching["F2"] || a.previewing == nil || a.previewing.index != 1 {
		t.Errorf("fetching %v, previewing %+v", a.fetching, a.previewing)
	}
}

// The cache keeps what was used last when it trims itself to its limit.
func TestTrimCache(t *testing.T) {
	a := newApp()
	a.cacheDir = t.TempDir()
	old, recent := a.imagePath("https://x/old"), a.imagePath("https://x/recent")
	keep(old, make([]byte, 600))
	keep(recent, make([]byte, 600))
	past := time.Now().Add(-time.Hour)
	os.Chtimes(old, past, past)
	a.trimCache(1000)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the file used least recently is left")
	}
	if data, ok := cached(recent); !ok || len(data) != 600 {
		t.Error("the file used last is gone")
	}
	a.clearCache()
	if a.cacheSize() != 0 {
		t.Error("the cache is not empty after clearing it")
	}
}
