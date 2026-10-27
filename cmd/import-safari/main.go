// Import-safari signs MySlack in with the session of Slack open in
// Safari: it reads the d cookie and the xoxc token of app.slack.com,
// checks them, and keeps them in the Keychain where the app looks.
//
//	go run ./cmd/import-safari [-team name]
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/egoist/mygo"

	"github.com/mrzzmrzz/MySlack/slack"
)

const identifier = "com.mrzz.myslack" // of mygo.json

func main() {
	team := flag.String("team", "", "name or domain of the workspace, when there are several")
	flag.Parse()
	home, _ := os.UserHomeDir()
	safari := filepath.Join(home, "Library/Containers/com.apple.Safari/Data/Library")

	cookie, err := slackCookie(filepath.Join(safari, "Cookies/Cookies.binarycookies"))
	if err != nil {
		log.Fatal(err)
	}
	teams, err := slackTeams(filepath.Join(safari, "WebKit/WebsiteData/Default"))
	if err != nil {
		log.Fatal(err)
	}
	var chosen *teamConfig
	for i, t := range teams {
		fmt.Printf("workspace: %s (%s)\n", t.Name, t.Domain)
		if *team == "" || strings.EqualFold(*team, t.Name) || strings.EqualFold(*team, t.Domain) {
			if chosen == nil {
				chosen = &teams[i]
			}
		}
	}
	if chosen == nil {
		log.Fatalf("no workspace %q", *team)
	}

	client := slack.New(chosen.Token, cookie)
	auth, err := client.AuthTest(context.Background())
	if err != nil {
		log.Fatalf("Safari's session does not work: %v", err)
	}
	data, _ := json.Marshal(map[string]string{"token": client.Token, "cookie": client.Cookie})
	if err := mygo.Secrets.Service(identifier).Set("session", data); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("signed in as %s on %s; open MySlack\n", auth.User, auth.Team)
}

// slackCookie reads the d cookie of .slack.com from Safari's cookie file.
func slackCookie(path string) (string, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(d) < 8 || string(d[:4]) != "cook" {
		return "", fmt.Errorf("%s: not a cookie file", path)
	}
	be, le := binary.BigEndian, binary.LittleEndian
	n := int(be.Uint32(d[4:]))
	off := 8 + 4*n
	for i := range n {
		size := int(be.Uint32(d[8+4*i:]))
		page := d[off : off+size]
		off += size
		count := int(le.Uint32(page[4:]))
		for j := range count {
			c := page[le.Uint32(page[8+4*j:]):]
			str := func(at int) string {
				s := c[le.Uint32(c[at:]):]
				return string(s[:bytes.IndexByte(s, 0)])
			}
			if strings.HasSuffix(str(16), "slack.com") && str(20) == "d" {
				return str(28), nil
			}
		}
	}
	return "", fmt.Errorf("no Slack session in Safari: sign in at app.slack.com first")
}

type teamConfig struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Token  string `json:"token"`
}

// slackTeams reads localConfig_v2 from app.slack.com's local storage.
func slackTeams(root string) ([]teamConfig, error) {
	origins, _ := filepath.Glob(filepath.Join(root, "*/*/origin"))
	for _, o := range origins {
		b, _ := os.ReadFile(o)
		if !bytes.Contains(b, []byte("app.slack.com")) {
			continue
		}
		db := filepath.Join(filepath.Dir(o), "LocalStorage/localstorage.sqlite3")
		out, err := exec.Command("sqlite3", "file:"+db+"?mode=ro",
			"select hex(value) from ItemTable where key='localConfig_v2'").Output()
		if err != nil || len(bytes.TrimSpace(out)) == 0 {
			continue
		}
		raw, err := hex.DecodeString(string(bytes.TrimSpace(out)))
		if err != nil {
			return nil, err
		}
		// WebKit keeps values as UTF-16LE.
		u := make([]uint16, len(raw)/2)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(raw[2*i:])
		}
		var cfg struct {
			Teams map[string]teamConfig `json:"teams"`
		}
		if err := json.Unmarshal([]byte(string(utf16.Decode(u))), &cfg); err != nil {
			return nil, err
		}
		var teams []teamConfig
		for _, t := range cfg.Teams {
			if strings.HasPrefix(t.Token, "xoxc-") {
				teams = append(teams, t)
			}
		}
		if len(teams) > 0 {
			return teams, nil
		}
	}
	return nil, fmt.Errorf("no Slack workspace in Safari's storage: open app.slack.com first")
}
