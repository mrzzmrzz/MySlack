# Slack-MyGo

A lightweight, unofficial Slack client for macOS, written in Go with
[MyGo](https://github.com/egoist/mygo)'s native UI. No Electron, no web view,
no JavaScript: the whole interface is drawn by MyGo on the GPU, styled after
macOS 27.

![Slack-MyGo](docs/screenshot.png)

## Features

- Channels, private channels, direct and group messages, in a sidebar of the
  system's material with avatars, as Messages shows people
- Messages in Slack's formatting: bold, italic, strikethrough, code, code
  blocks, quotes, lists, links, mentions and emoji shortcodes
- Real-time updates over Slack's socket: new messages, edits, deletions,
  replies and reactions
- Threads in an inspector beside the channel
- Reactions: add any emoji from a searchable picker, toggle them in place
- Files: download to Downloads with progress, open or reveal in Finder;
  pictures as thumbnails that open in Preview
- Desktop notifications for direct messages and mentions, and an unread
  count on the Dock
- Settings (⌘,): appearance, text size, compact mode, ⌘↩ to send,
  notifications
- Session kept in the macOS Keychain

![Reactions](docs/reactions.png)

## Install

Download the disk image from the
[latest release](https://github.com/mrzzmrzz/Slack-MyGo/releases/latest),
open it and drag Slack-MyGo to Applications.

Releases built without a Developer ID are signed ad hoc: the first time,
right-click the app and choose Open, or allow it under System Settings ›
Privacy & Security.

## Sign in

Slack-MyGo signs in with the session of Slack's web client: an `xoxc-` token
and the `d` cookie that goes with it.

- **From Safari**: sign in at [app.slack.com](https://app.slack.com) in
  Safari, then run

  ```sh
  go run ./cmd/import-safari            # or -team <name> with several workspaces
  ```

  It reads the session from Safari, checks it, and keeps it in the Keychain
  where the app finds it. Your terminal needs Full Disk Access to read
  Safari's data.

- **By hand**: in the browser's developer tools on app.slack.com, run
  `Object.values(JSON.parse(localStorage.localConfig_v2).teams)[0].token`
  in the console for the token, and copy the `d` cookie from the cookies
  of `slack.com`. Paste both into the app's sign-in window.

> [!WARNING]
> Using a session token outside Slack's own clients may be against Slack's
> terms of service, and your workspace's administrators may not allow it.
> Use it at your own risk, on workspaces where you may.

## Build

Requires Go 1.27.3 or later, on macOS.

```sh
go tool mygo dev      # run with live reload
CGO_ENABLED=0 go test ./...
go tool mygo build    # build/darwin-<arch>/Slack-MyGo.app and its .dmg
```

The code is small:

| Path | What |
|---|---|
| `slack/` | Web API client, real-time socket, mrkdwn to Markdown, emoji table |
| `view.go` | the main window |
| `settings.go` | the settings window |
| `avatar.go` | avatars |
| `main.go` | state, sign-in, events, notifications, menus |
| `cmd/import-safari` | imports the session from Safari |

## Releases

Pushing a tag `v<version>` matching `version` in `mygo.json` builds a
universal disk image and publishes it as a GitHub release. With these
repository secrets set, the app is signed with a Developer ID and notarized:

| Secret | |
|---|---|
| `APPLE_CERTIFICATE_BASE64` | the Developer ID Application certificate, a `.p12` in base64 |
| `APPLE_CERTIFICATE_PASSWORD` | its password |
| `APPLE_SIGNING_IDENTITY` | e.g. `Developer ID Application: Jane Doe (TEAMID)` |
| `APPLE_NOTARY_KEY` | an App Store Connect API key, the `.p8` file or it in base64 |
| `APPLE_NOTARY_KEY_ID` | its key ID |
| `APPLE_NOTARY_ISSUER_ID` | its issuer ID |

## License

[MIT](LICENSE).

Slack and the Slack logo are trademarks of Slack Technologies, LLC, a
Salesforce company. This project is not affiliated with, endorsed by or
sponsored by Slack or Salesforce.
