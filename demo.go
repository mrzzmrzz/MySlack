package main

import (
	"fmt"
	"time"

	"github.com/mrzzmrzz/MySlack/slack"
)

// loadDemo fills the app with a workspace of made-up people and messages,
// signed in and connected but talking to no server: for screenshots, with
// SLACK_MYGO_DEMO set. "thread" opens a thread beside the channel.
func (a *app) loadDemo(mode string) {
	a.auth = &slack.Auth{Team: "Acme Labs", User: "me", UserID: "U00", URL: "https://acme.slack.com/"}
	a.connected = true
	for id, name := range map[string]string{
		"U00": "Alex Chen", "U01": "Ada Lovelace", "U02": "Grace Hopper", "U03": "Linus Park",
		"U04": "Margaret Hamilton", "U05": "Katherine Johnson", "U06": "Dennis Ritchie",
	} {
		u := &slack.User{ID: id, Name: name, TZ: "America/Los_Angeles"}
		u.Profile.DisplayName = name
		a.users[id] = u
	}
	a.users["U03"].Profile.Title, a.users["U03"].TZ = "Release engineer", "Europe/Berlin"
	a.users["U03"].Profile.StatusEmoji, a.users["U03"].Profile.StatusText = ":rocket:", "Shipping v0.2"
	a.users["U04"].Profile.Title = "Build systems"
	add := func(ch slack.Channel, topic string, unread bool, ms ...slack.Message) *channel {
		ch.Topic.Value = topic
		c := &channel{Channel: ch, loaded: true, unread: unread, messages: ms}
		a.channels[ch.ID] = c
		return c
	}
	at := func(d time.Time, h, m int) string {
		t := time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, time.Local)
		return fmt.Sprintf("%d.000100", t.Unix())
	}
	today := time.Now()

	add(slack.Channel{ID: "C01", Name: "general", IsChannel: true}, "Company-wide announcements", false)
	add(slack.Channel{ID: "C02", Name: "design", IsChannel: true}, "", true)
	add(slack.Channel{ID: "C04", Name: "random", IsChannel: true}, "", false)
	add(slack.Channel{ID: "C05", Name: "launch-team", IsChannel: true, IsPrivate: true}, "", true)
	eng := add(slack.Channel{ID: "C03", Name: "engineering", IsChannel: true},
		"Ship small, ship often · on call this week: @Grace", false,
		slack.Message{User: "U03", TS: at(today, 9, 41), ReplyCount: 3,
			Text:      "Release checklist for *v0.2* :rocket:\n• tag from `main`\n• let CI build the universal DMG\n• write the notes",
			Reactions: []slack.Reaction{{Name: "rocket", Count: 4, Users: []string{"U00", "U01", "U02", "U04"}}, {Name: "eyes", Count: 1, Users: []string{"U05"}}}},
		slack.Message{User: "U04", TS: at(today, 10, 15), Text: "Here is the config the build reads:\n```{\n  \"name\": \"MySlack\",\n  \"version\": \"0.2.0\"\n}```\n&gt; Keep the tag and this version in step."},
		slack.Message{User: "U05", TS: at(today, 10, 32), Text: "Spec for the new composer, comments welcome <@U00>",
			Files: []slack.File{{ID: "F01", Name: "composer-spec.pdf", Title: "composer-spec.pdf", Filetype: "PDF", Size: 2_400_000}}},
		slack.Message{User: "U00", TS: at(today, 10, 41), Text: "Looks great, I'll take the keyboard shortcuts :sparkles:",
			Reactions: []slack.Reaction{{Name: "heart", Count: 2, Users: []string{"U05", "U01"}}}},
	)
	add(slack.Channel{ID: "D01", IsIM: true, User: "U01"}, "", true)
	add(slack.Channel{ID: "D02", IsIM: true, User: "U02"}, "", false)
	add(slack.Channel{ID: "D03", IsIM: true, User: "U04"}, "", false)
	add(slack.Channel{ID: "D04", IsIM: true, User: "U06"}, "", false)
	add(slack.Channel{ID: "G01", Name: "mpdm-ada--grace--linus-1", IsMPIM: true}, "", false)

	a.selected = eng.ID
	if mode == "thread" {
		parent := eng.messages[0]
		a.thread = &thread{channel: eng.ID, ts: parent.TS, messages: []slack.Message{
			parent,
			{User: "U02", TS: at(today, 9, 50), ThreadTS: parent.TS, Text: "I can write the notes."},
			{User: "U06", TS: at(today, 9, 55), ThreadTS: parent.TS, Text: "CI is green on `main` :white_check_mark:"},
			{User: "U03", TS: at(today, 10, 2), ThreadTS: parent.TS, Text: "Thanks both, tagging after lunch."},
		}}
	}
}
