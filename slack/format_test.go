package slack

import "testing"

func TestPlainText(t *testing.T) {
	names := map[string]string{"U1": "ada", "C1": "general"}
	got := PlainText("hi <@U1> in <#C1> and <#C2|random>, <!here>: <https://x.dev|docs> &amp; <https://y.dev>", func(id string) string { return names[id] })
	want := "hi @ada in #general and #random, @here: docs (https://x.dev) & https://y.dev"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
