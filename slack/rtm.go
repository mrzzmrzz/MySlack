package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/net/websocket"
)

// Event is a message or reaction event of the real-time socket. Changed
// holds the new version of an edited message, DeletedTS the message a
// deletion removed; Reaction and Item the reaction added or removed, by
// User, and its message.
type Event struct {
	Message
	Channel   string   `json:"channel"`
	Changed   *Message `json:"message"`
	DeletedTS string   `json:"deleted_ts"`
	Reaction  string   `json:"reaction"`
	Item      struct {
		Channel string `json:"channel"`
		TS      string `json:"ts"`
	} `json:"item"`
}

// Listen keeps a real-time connection open until ctx ends, reconnecting
// after failures, and hands each message event to fn on its goroutine.
// status reports whether the socket is connected.
func (c *Client) Listen(ctx context.Context, fn func(Event), status func(connected bool, err error)) {
	backoff := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := c.listenOnce(ctx, fn, status)
		if ctx.Err() != nil {
			return
		}
		status(false, err)
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff = min(backoff*2, time.Minute)
	}
}

func (c *Client) listenOnce(ctx context.Context, fn func(Event), status func(bool, error)) error {
	var r struct {
		URL string `json:"url"`
	}
	if err := c.call(ctx, "rtm.connect", url.Values{"batch_presence_aware": {"1"}}, &r); err != nil {
		return err
	}
	cfg, err := websocket.NewConfig(r.URL, "https://app.slack.com")
	if err != nil {
		return err
	}
	cfg.Header.Set("Cookie", c.cookieHeader())
	ws, err := cfg.DialContext(ctx)
	if err != nil {
		return err
	}
	defer ws.Close()
	stop := context.AfterFunc(ctx, func() { ws.Close() })
	defer stop()

	// Pings keep the socket alive; a socket silent for long is dead.
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for id := 1; ; id++ {
			select {
			case <-t.C:
				if websocket.Message.Send(ws, fmt.Sprintf(`{"id":%d,"type":"ping"}`, id)) != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		ws.SetReadDeadline(time.Now().Add(90 * time.Second))
		var data []byte
		if err := websocket.Message.Receive(ws, &data); err != nil {
			return err
		}
		var head struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &head) != nil {
			continue
		}
		switch head.Type {
		case "hello":
			status(true, nil)
		case "goodbye":
			return fmt.Errorf("server closed the connection")
		case "message", "reaction_added", "reaction_removed":
			var e Event
			if json.Unmarshal(data, &e) == nil {
				fn(e)
			}
		}
	}
}
