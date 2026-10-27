package main

import (
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// The tests read the app in Simplified Chinese, whatever the machine's
// language: its texts are what they check.
func init() { i18n.SetLocaleFunc(func() string { return "zh-Hans" }) }

// The view says its messages in the language the app shows, and follows a
// change of it.
func TestLanguageFollowsTheApp(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 900, 600)
	if !tt.HasText("登录 Slack") {
		t.Fatalf("texts %q", tt.Texts())
	}
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "zh-Hans" }) })
	tt.SetLocale("en")
	tt.Frame()
	if !tt.HasText("Sign in to Slack") || tt.HasText("登录 Slack") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if got := a.menu().Items()[0].Submenu[2].Label; got != "Settings…" {
		t.Errorf("menu label %q", got)
	}
}
