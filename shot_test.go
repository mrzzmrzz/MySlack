package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/mrzzmrzz/MySlack/slack"
)

func TestShot(t *testing.T) {
	dir := os.Getenv("SHOT_DIR")
	if dir == "" {
		t.Skip()
	}
	a := newApp()
	save := func(name string, tt *ui.Tester) {
		f, _ := os.Create(dir + "/" + name)
		png.Encode(f, tt.Image())
		f.Close()
	}
	save("signin.png", ui.NewTester(a.view, 1100, 720))
	a.auth = &slack.Auth{Team: "Acme", User: "me", UserID: "U0"}
	a.connected = true
	a.users["U1"] = &slack.User{ID: "U1", Name: "ada"}
	a.users["U2"] = &slack.User{ID: "U2", Name: "bob"}
	// bob has a picture of his own, a photo-like gradient.
	img := image.NewRGBA(image.Rect(0, 0, 72, 72))
	for y := 0; y < 72; y++ {
		for x := 0; x < 72; x++ {
			img.Set(x, y, color.RGBA{uint8(200 - y), uint8(150 + x/2), 220, 255})
		}
	}
	a.users["U2"].Profile.IsCustomImage, a.users["U2"].Profile.Image72 = true, "bob.png"
	a.images["bob.png"] = &picture{bitmap: ui.NewBitmap(img)}
	a.client = slack.New("xoxc-shot", "d")
	a.channels["C1"] = &channel{Channel: slack.Channel{ID: "C1", Name: "general", IsChannel: true}, loaded: true,
		messages: []slack.Message{
			{User: "U1", Text: "早上好 <@U2>，今天发布 <https://x.dev|v1.2> 吗？ :rocket:", TS: "1700000000.000100", ReplyCount: 2,
				Reactions: []slack.Reaction{{Name: "+1", Count: 2, Users: []string{"U0", "U2"}}, {Name: "eyes", Count: 1, Users: []string{"U2"}}}},
			{User: "U1", Text: "顺便看一下 *CI* 的 _flaky_ 测试，跑 `go test ./...` 就行", TS: "1700000030.000100"},
			{User: "U2", Text: "可以，下午三点。配置如下：\n```deploy:\n  region: us-east-1\n  replicas: 3```\n&gt; 记得先备份数据库\n• 第一步\n• 第二步", TS: "1700090100.000100"},
		}}
	a.channels["C2"] = &channel{Channel: slack.Channel{ID: "C2", Name: "random", IsChannel: true}, unread: true}
	a.channels["D1"] = &channel{Channel: slack.Channel{ID: "D1", IsIM: true, User: "U2"}}
	a.selected = "C1"
	tt := ui.NewTester(a.view, 1100, 720)
	tt.Frame()
	save("main.png", tt)
	a.thread = &thread{channel: "C1", ts: "1700000000.000100", messages: a.channels["C1"].messages[:1]}
	tt.Frame()
	save("thread.png", tt)
	a.picking, a.pickerOpen = "1700090100.000100", true
	tt.Frame()
	tt.Frame()
	save("picker.png", tt)
	st := ui.NewTester(a.settingsView, 540, 470)
	save("settings.png", st)
	a.settingsTab = tabNotifications
	st.Frame()
	save("settings-notify.png", st)
}
