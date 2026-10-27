package slack

import "testing"

func TestMarkdown(t *testing.T) {
	names := map[string]string{"U1": "ada", "C1": "general"}
	name := func(id string) string { return names[id] }
	for _, tc := range []struct{ in, want string }{
		{"*bold* _it_ ~gone~ x*y*z", "**bold** *it* ~~gone~~ x\\*y\\*z"},
		{"hi <@U1> in <#C1>", "hi [@ada](slack://user/U1) in [#general](slack://channel/C1)"},
		{"<https://x.dev/a_b|the *docs*> and <https://y.dev>", "[the \\*docs\\*](<https://x.dev/a_b>) and [https://y.dev](<https://y.dev>)"},
		{":+1::skin-tone-2: :party_parrot: :smile:", "👍🏻 :party\\_parrot: 😄"},
		{"&gt; quoted\n&gt; more\nnext", "> quoted  \n> more\n\nnext"},
		{"run `a_b *c*` now", "run `a_b *c*` now"},
		{"```if a &lt; b {\n}```after", "```\nif a < b {\n}\n```\n\nafter"},
		{"# not a heading\n<!here> look", "\\# not a heading  \n**@here** look"},
		{"中文*粗体*测试", "中文\\*粗体\\*测试"},
		{"中文 *粗体* 测试", "中文 **粗体** 测试"},
	} {
		if got := Markdown(tc.in, name); got != tc.want {
			t.Errorf("Markdown(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}
