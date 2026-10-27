package slack

import (
	"html"
	"regexp"
	"strings"
)

var entity = regexp.MustCompile(`<([^<>]+)>`)

// PlainText turns Slack's mrkdwn into text to read: mentions become
// names, links their labels or addresses. name looks up users and
// channels by ID, returning "" for those it does not know.
func PlainText(s string, name func(id string) string) string {
	s = entity.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1]
		target, label, _ := strings.Cut(inner, "|")
		switch {
		case strings.HasPrefix(target, "@"):
			if n := name(target[1:]); n != "" {
				return "@" + n
			}
			if label != "" {
				return "@" + label
			}
			return target
		case strings.HasPrefix(target, "#"):
			if label != "" {
				return "#" + label
			}
			if n := name(target[1:]); n != "" {
				return "#" + n
			}
			return target
		case strings.HasPrefix(target, "!"):
			if label != "" {
				return label
			}
			cmd, _, _ := strings.Cut(target[1:], "^")
			return "@" + cmd
		}
		if label != "" && label != target {
			return label + " (" + target + ")"
		}
		return target
	})
	return html.UnescapeString(s)
}

// Body is the text a message shows, falling back on its attachments and
// files when it has none.
func (m *Message) Body() string {
	parts := []string{}
	if m.Text != "" {
		parts = append(parts, m.Text)
	}
	for _, a := range m.Attachments {
		switch {
		case a.Text != "" || a.Pretext != "" || a.Title != "":
			for _, s := range []string{a.Pretext, a.Title, a.Text} {
				if s != "" && s != m.Text {
					parts = append(parts, s)
				}
			}
		case a.Fallback != "" && a.Fallback != m.Text:
			parts = append(parts, a.Fallback)
		}
	}
	for _, f := range m.Files {
		n := f.Title
		if n == "" {
			n = f.Name
		}
		parts = append(parts, "📎 "+n)
	}
	return strings.Join(parts, "\n")
}
