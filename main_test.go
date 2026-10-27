package main

import (
	"testing"

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
