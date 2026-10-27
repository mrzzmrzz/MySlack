// Package slack talks to Slack's Web API and real-time socket with the
// session credentials of the web client: an xoxc token and its d cookie.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	Token  string // xoxc-…
	Cookie string // value of the d cookie, xoxd-…
	base   string // https://<team>.slack.com/api/ once auth.test answers
	http   *http.Client
}

func New(token, cookie string) *Client {
	return &Client{
		Token:  strings.TrimSpace(token),
		Cookie: strings.TrimPrefix(strings.TrimSpace(cookie), "d="),
		base:   "https://slack.com/api/",
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) cookieHeader() string { return "d=" + c.Cookie }

// call posts a Web API method and decodes the answer into out, which
// must embed nothing: ok and error are checked here.
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("token", c.Token)
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", c.base+method, strings.NewReader(params.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", c.cookieHeader())
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			resp.Body.Close()
			wait := 2 * time.Second
			if s, err := time.ParseDuration(resp.Header.Get("Retry-After") + "s"); err == nil {
				wait = s
			}
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		var raw json.RawMessage
		err = json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("%s: HTTP %d: %w", method, resp.StatusCode, err)
		}
		var status struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(raw, &status); err != nil {
			return err
		}
		if !status.OK {
			return fmt.Errorf("%s: %s", method, status.Error)
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(raw, out)
	}
}

type Auth struct {
	URL    string `json:"url"`
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
}

// AuthTest checks the credentials and points later calls at the team's
// own domain, which some workspaces require.
func (c *Client) AuthTest(ctx context.Context) (*Auth, error) {
	var a Auth
	if err := c.call(ctx, "auth.test", nil, &a); err != nil {
		return nil, err
	}
	if a.URL != "" {
		c.base = strings.TrimSuffix(a.URL, "/") + "/api/"
	}
	return &a, nil
}

type Channel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsChannel bool   `json:"is_channel"`
	IsGroup   bool   `json:"is_group"`
	IsPrivate bool   `json:"is_private"`
	IsIM      bool   `json:"is_im"`
	IsMPIM    bool   `json:"is_mpim"`
	User      string `json:"user"` // the other person of an IM
	Topic     struct {
		Value string `json:"value"`
	} `json:"topic"`
	Purpose struct {
		Value string `json:"value"`
	} `json:"purpose"`
}

type cursorMeta struct {
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// Conversations lists the conversations the user is a member of.
func (c *Client) Conversations(ctx context.Context) ([]Channel, error) {
	var all []Channel
	cursor := ""
	for {
		var r struct {
			Channels []Channel `json:"channels"`
			cursorMeta
		}
		p := url.Values{
			"types":            {"public_channel,private_channel,mpim,im"},
			"exclude_archived": {"true"},
			"limit":            {"999"},
		}
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		if err := c.call(ctx, "users.conversations", p, &r); err != nil {
			return nil, err
		}
		all = append(all, r.Channels...)
		cursor = r.ResponseMetadata.NextCursor
		if cursor == "" {
			return all, nil
		}
	}
}

type File struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Mimetype  string `json:"mimetype"`
	Permalink string `json:"permalink"`
	Thumb360  string `json:"thumb_360"`
	Thumb360W int    `json:"thumb_360_w"`
	Thumb360H int    `json:"thumb_360_h"`
}

type Reaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

type Attachment struct {
	Fallback string `json:"fallback"`
	Pretext  string `json:"pretext"`
	Text     string `json:"text"`
	Title    string `json:"title"`
}

type Message struct {
	Type        string       `json:"type"`
	Subtype     string       `json:"subtype"`
	User        string       `json:"user"`
	BotID       string       `json:"bot_id"`
	Username    string       `json:"username"`
	Text        string       `json:"text"`
	TS          string       `json:"ts"`
	ThreadTS    string       `json:"thread_ts"`
	ReplyCount  int          `json:"reply_count"`
	Files       []File       `json:"files"`
	Attachments []Attachment `json:"attachments"`
	Reactions   []Reaction   `json:"reactions"`
	Edited      *struct {
		TS string `json:"ts"`
	} `json:"edited"`
}

// Time is when the message was sent.
func (m *Message) Time() time.Time {
	sec, frac, _ := strings.Cut(m.TS, ".")
	var s, us int64
	fmt.Sscan(sec, &s)
	fmt.Sscan(frac, &us)
	return time.Unix(s, us*1000)
}

// IsReply reports a reply in a thread that does not show in the channel.
func (m *Message) IsReply() bool {
	return m.ThreadTS != "" && m.ThreadTS != m.TS && m.Subtype != "thread_broadcast"
}

// History returns the latest messages of a channel, oldest first.
func (c *Client) History(ctx context.Context, channel string, limit int) ([]Message, error) {
	var r struct {
		Messages []Message `json:"messages"`
	}
	p := url.Values{"channel": {channel}, "limit": {fmt.Sprint(limit)}}
	if err := c.call(ctx, "conversations.history", p, &r); err != nil {
		return nil, err
	}
	reverse(r.Messages)
	return r.Messages, nil
}

// Replies returns a thread, its parent first.
func (c *Client) Replies(ctx context.Context, channel, ts string) ([]Message, error) {
	var all []Message
	cursor := ""
	for {
		var r struct {
			Messages []Message `json:"messages"`
			cursorMeta
		}
		p := url.Values{"channel": {channel}, "ts": {ts}, "limit": {"200"}}
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		if err := c.call(ctx, "conversations.replies", p, &r); err != nil {
			return nil, err
		}
		all = append(all, r.Messages...)
		cursor = r.ResponseMetadata.NextCursor
		if cursor == "" {
			return all, nil
		}
	}
}

func reverse(ms []Message) {
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
}

type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
	Profile struct {
		DisplayName string `json:"display_name"`
		RealName    string `json:"real_name"`
		Image72     string `json:"image_72"`
		// IsCustomImage is false for Slack's default picture, a
		// silhouette that reads badly when cropped round and small.
		IsCustomImage bool `json:"is_custom_image"`
	} `json:"profile"`
}

// Photo is the URL of the user's own picture, "" for Slack's default.
func (u *User) Photo() string {
	if !u.Profile.IsCustomImage {
		return ""
	}
	return u.Profile.Image72
}

// Label is the name Slack shows for the user.
func (u *User) Label() string {
	switch {
	case u.Profile.DisplayName != "":
		return u.Profile.DisplayName
	case u.Profile.RealName != "":
		return u.Profile.RealName
	}
	return u.Name
}

func (c *Client) UserInfo(ctx context.Context, id string) (*User, error) {
	var r struct {
		User User `json:"user"`
	}
	if err := c.call(ctx, "users.info", url.Values{"user": {id}}, &r); err != nil {
		return nil, err
	}
	return &r.User, nil
}

// Post sends text to a channel, into a thread when threadTS is set.
func (c *Client) Post(ctx context.Context, channel, text, threadTS string) error {
	p := url.Values{"channel": {channel}, "text": {text}}
	if threadTS != "" {
		p.Set("thread_ts", threadTS)
	}
	return c.call(ctx, "chat.postMessage", p, nil)
}

// Mark marks a channel read up to ts.
func (c *Client) Mark(ctx context.Context, channel, ts string) error {
	return c.call(ctx, "conversations.mark", url.Values{"channel": {channel}, "ts": {ts}}, nil)
}

// Unreads returns which conversations have unread messages, by the
// web client's own method; it is not part of the public API.
func (c *Client) Unreads(ctx context.Context) (map[string]bool, error) {
	type entry struct {
		ID         string `json:"id"`
		HasUnreads bool   `json:"has_unreads"`
	}
	var r struct {
		Channels []entry `json:"channels"`
		MPIMs    []entry `json:"mpims"`
		IMs      []entry `json:"ims"`
	}
	if err := c.call(ctx, "client.counts", nil, &r); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, list := range [][]entry{r.Channels, r.MPIMs, r.IMs} {
		for _, e := range list {
			if e.HasUnreads {
				out[e.ID] = true
			}
		}
	}
	return out, nil
}

// React adds the user's reaction to a message, or removes it.
func (c *Client) React(ctx context.Context, channel, ts, name string, add bool) error {
	method := "reactions.remove"
	if add {
		method = "reactions.add"
	}
	return c.call(ctx, method, url.Values{"channel": {channel}, "timestamp": {ts}, "name": {name}}, nil)
}

// Fetch downloads a file of the workspace, as a thumbnail, which needs
// the session to read.
func (c *Client) Fetch(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Cookie", c.cookieHeader())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
}
