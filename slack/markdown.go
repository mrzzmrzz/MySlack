package slack

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Emoji returns the character of an emoji's short name, as "+1" or
// "smile::skin-tone-2", or "" for a name it does not know, as a
// workspace's own emoji.
func Emoji(name string) string {
	base, tone, _ := strings.Cut(name, "::")
	e, ok := emojiTable[base]
	if !ok {
		return ""
	}
	return e + emojiTable[tone]
}

var (
	inlineCode = regexp.MustCompile("`[^`\n]+`")
	shortcode  = regexp.MustCompile(`:([a-z0-9_+'-]+):(?::(skin-tone-[2-6]):)?`)
)

// Markdown turns Slack's mrkdwn into CommonMark: *bold*, _italic_,
// ~strike~, code, quotes, links, and mentions as links to
// slack://user/<id> and slack://channel/<id>. name looks up users and
// channels by ID, returning "" for those it does not know.
func Markdown(s string, name func(id string) string) string {
	var b strings.Builder
	parts := strings.Split(s, "```")
	if len(parts)%2 == 0 { // an unclosed fence is text
		parts[len(parts)-2] += "```" + parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}
	for i, p := range parts {
		if i%2 == 1 {
			code := strings.Trim(codeText(p), "\n")
			b.WriteString("\n\n```\n" + code + "\n```\n\n")
			continue
		}
		b.WriteString(blockText(p, name))
	}
	return strings.TrimSpace(b.String())
}

// codeText is the text of code, whose links and entities Slack still
// marks up.
func codeText(s string) string {
	s = entity.ReplaceAllStringFunc(s, func(m string) string {
		target, label, _ := strings.Cut(m[1:len(m)-1], "|")
		if label != "" {
			return label
		}
		return target
	})
	return html.UnescapeString(s)
}

// blockText converts text outside code blocks, line by line.
func blockText(s string, name func(string) string) string {
	lines := strings.Split(s, "\n")
	quoted := make([]bool, len(lines))
	for i, line := range lines {
		quote := ""
		if rest, ok := strings.CutPrefix(line, "&gt;"); ok {
			quote, line = "> ", strings.TrimPrefix(rest, " ")
			quoted[i] = true
		}
		// Indented text is not code in Slack.
		trimmed := strings.TrimLeft(line, " \t")
		indent := strings.Repeat(" ", len(line)-len(trimmed))
		text := inlineText(trimmed, name)
		if strings.HasPrefix(text, "#") {
			text = `\` + text
		}
		lines[i] = quote + indent + text
	}
	// Each line break of Slack is a hard break; blank lines part
	// paragraphs, and a quote ends where its lines do.
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(line)
		if i == len(lines)-1 {
			break
		}
		next := lines[i+1]
		switch {
		case quoted[i] && !quoted[i+1]:
			b.WriteString("\n")
		case line != "" && next != "" && strings.TrimSpace(line) != ">":
			b.WriteString("  ")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// inlineText converts a line: its code spans are kept, the rest has
// its entities and emoji replaced and its emphasis converted.
func inlineText(s string, name func(string) string) string {
	var b strings.Builder
	last := 0
	for _, m := range inlineCode.FindAllStringIndex(s, -1) {
		b.WriteString(prose(s[last:m[0]], name))
		b.WriteString("`" + codeText(s[m[0]+1:m[1]-1]) + "`")
		last = m[1]
	}
	b.WriteString(prose(s[last:], name))
	return b.String()
}

// prose converts text without code. Entities and emoji become
// placeholders first so that emphasis does not reach into them.
func prose(s string, name func(string) string) string {
	var held []string
	hold := func(md string) string {
		held = append(held, md)
		return fmt.Sprintf("\x00%d\x00", len(held)-1)
	}
	s = entity.ReplaceAllStringFunc(s, func(m string) string { return hold(link(m[1:len(m)-1], name)) })
	s = shortcode.ReplaceAllStringFunc(s, func(m string) string {
		sub := shortcode.FindStringSubmatch(m)
		e := Emoji(sub[1])
		if e == "" {
			return hold(escape(m))
		}
		return hold(e + emojiTable[sub[2]])
	})
	s = emphasis(s)
	for i, md := range held {
		s = strings.Replace(s, fmt.Sprintf("\x00%d\x00", i), md, 1)
	}
	return s
}

func link(inner string, name func(string) string) string {
	target, label, _ := strings.Cut(inner, "|")
	switch {
	case strings.HasPrefix(target, "@"):
		n := name(target[1:])
		if n == "" {
			n = label
		}
		if n == "" {
			n = target[1:]
		}
		return "[@" + escape(n) + "](slack://user/" + target[1:] + ")"
	case strings.HasPrefix(target, "#"):
		n := label
		if n == "" {
			n = name(target[1:])
		}
		if n == "" {
			n = target[1:]
		}
		return "[#" + escape(n) + "](slack://channel/" + target[1:] + ")"
	case strings.HasPrefix(target, "!"):
		if label != "" {
			return "**" + escape(html.UnescapeString(label)) + "**"
		}
		cmd, _, _ := strings.Cut(target[1:], "^")
		return "**@" + cmd + "**"
	}
	if label == "" {
		label = target
	}
	return "[" + escape(html.UnescapeString(label)) + "](<" + target + ">)"
}

// escape keeps text from reading as Markdown.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("\\`*_~[]<>|", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

var marks = map[byte]string{'*': "**", '_': "*", '~': "~~"}

// emphasis converts Slack's *bold*, _italic_ and ~strike~, which open
// after a non-word character and close before one, and escapes every
// other mark Markdown would read.
func emphasis(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if md, ok := marks[c]; ok && opens(s, i) {
			if j := closes(s, i); j > 0 {
				b.WriteString(md + emphasis(s[i+1:j]) + md)
				i = j + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if strings.ContainsRune("\\`*_~[]<|", r) {
			b.WriteByte('\\')
		}
		b.WriteString(s[i : i+size])
		i += size
	}
	return b.String()
}

func opens(s string, i int) bool {
	if i+1 >= len(s) || s[i+1] == ' ' || s[i+1] == s[i] {
		return false
	}
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !isWord(r)
}

// closes finds the mark closing the one at i, or returns -1.
func closes(s string, i int) int {
	for j := i + 2; j < len(s); j++ {
		if s[j] != s[i] || s[j-1] == ' ' {
			continue
		}
		if j+1 == len(s) {
			return j
		}
		if r, _ := utf8.DecodeRuneInString(s[j+1:]); !isWord(r) {
			return j
		}
	}
	return -1
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// emojiNames are the short names of emojiTable, the shortest first, so
// that searches find "smile" before "smiley_cat".
var emojiNames = func() []string {
	var names []string
	seen := map[string]bool{}
	for n, e := range emojiTable {
		if strings.HasPrefix(n, "skin-tone-") || seen[e] {
			continue
		}
		seen[e] = true
		names = append(names, n)
	}
	slices.SortFunc(names, func(a, b string) int {
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return strings.Compare(a, b)
	})
	return names
}()

// SearchEmoji returns up to limit short names of emoji containing query,
// those starting with it first, one name for each emoji.
func SearchEmoji(query string, limit int) []string {
	query = strings.ToLower(strings.Trim(query, ": "))
	var first, rest []string
	for _, n := range emojiNames {
		switch {
		case strings.HasPrefix(n, query):
			first = append(first, n)
		case strings.Contains(n, query):
			rest = append(rest, n)
		}
		if len(first) >= limit {
			break
		}
	}
	return append(first, rest...)[:min(limit, len(first)+len(rest))]
}
