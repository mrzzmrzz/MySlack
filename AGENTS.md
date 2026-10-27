# AGENTS.md

Guidance for agents and people changing this app. MySlack and its sibling
MyDiscord share one design language: keep them alike.

## The aim

A small, elegant, native macOS 27 app. It should feel like something Apple
might ship beside Mail and Messages, not like the service's own client
re-skinned. When the service's design and the platform's disagree, follow
the platform.

## Design rules

### Window and structure

- One window: a **sidebar**, the **content**, and an optional **inspector**
  at the right (threads, members, a profile). Nothing else in the chrome.
- The sidebar uses the system's material (`Vibrancy: mygo.VibrancySidebar`),
  runs along the window's whole left edge, and sits under the traffic lights
  (`TitleBarStyle: mygo.TitleBarHiddenInset`). Do not float it as a card or
  inset it.
- The sidebar collapses: a button beside the traffic lights and ⌃⌘S in the
  View (显示) menu, sliding out over 250 ms. The toolbar's title then moves
  clear of the window's controls.
- The toolbar is 52 points tall, frosted under the content as it scrolls
  (`glass.ScrollEdge{Hard: true}`), and draggable. Its title is **title over
  subtitle** (14 pt semibold over 11 pt secondary), as Mail and Notes do;
  no icons or vertical dividers beside it.
- Inspectors are full height with a hairline at their edge, not floating
  panes. The button that toggles one uses the inspector glyph (a rectangle
  with a pane at its right).

### Shapes: circles for identities, concentric for controls

- **Every identity is a circle**: people's avatars, servers and workspaces,
  channels. Never mix rounded squares with circles in one list.
- Channels show as a white glyph (`#`, a lock, a megaphone) on a circle of a
  system color, lighter at the top, as Reminders' lists. The color is taken
  from the channel's ID, from the system colors; never pale ones that the
  white glyph would not read on.
- People without a picture of their own show their initials on a gradient of
  their name's hue. Slack's default silhouette counts as no picture. Draw
  avatars with `avatar()` in `avatar.go`, never `ui.Avatar`, which draws
  pictures at its own size whatever the element's.
- **Controls are concentric with what holds them.** A button inside a
  capsule is a circle whose radius is the capsule's minus its padding: the
  composer (radius 18, padding 4) holds 28-point circles; the hover bar
  (radius 17, padding 3) holds 28-point circles. Icon buttons are round
  (radius 14 at 28 points) everywhere, toolbars included.
- Capsules for things you can press that carry a count or a short label:
  reactions, the search field, the composer.

### Glass

- Liquid Glass (`plugins/glass`) only for what floats over content: the
  composer, the hover bar of a message, the send button (tinted with the
  accent when it can send). Not for panes, cards or lists.
- Absolute layers paint over the ones in the flow, whatever their order. A
  card over a backdrop (the sign-in's blobs and blur) must be absolute too, or
  the blur frosts it away. Offscreen renders show no materials: look at the
  real window.
- Content scrolls under the toolbar and under the composer; their edges
  frost or fade it (`glass.ScrollEdge`).

### Color and type

- System colors only: the theme's text, secondary text, accent; status
  green `#34C759`, orange `#FF9F0A`, red `#FF3B30`, gray `#8E8E93`; badges
  red. No brand or role colors on names; names are the text color.
- **No colored bars or stripes** along cards, quotes or messages. Link
  previews are plain rounded cards; a small dot of the site's color marks
  the site.
- No tinted backgrounds behind whole messages, which run together when
  several follow each other. Mark a mention of the user by making the
  mention itself bold.
- The "new messages" divider is a hairline and label in the accent color.
- System font. Body 13 pt; names 13 pt semibold; times and secondary
  lines 11 pt; section titles 11 pt semibold secondary.

### Messages

- Author and time (time only, `15:04`; the day is in the divider above),
  then the text. A message following the same author within minutes leaves
  out both; its time shows on hover.
- Day dividers are centered secondary text, without a capsule.
- Joins, topic changes and the like are one quiet line of secondary text.
- A reply shows what it answers as one secondary line above it ("↩ 回复
  某人：…"), never a connector drawn to the other message.
- Hover shows a capsule of glass at the message's top corner with its
  actions; a menu (⋯) holds the rest.
- Editing happens in place, with 取消 and 存储 buttons under it.
- Empty states are centered in the content: a large glyph or face, a title,
  one line of secondary text.

### Menus, popovers, sheets

- Choices from a short list use the **system's menus** (`Element.Menu`):
  switching workspace or server, the account's status, the microphone and
  sound, settings. Not hand-drawn cards.
- Popovers for content that is not a choice: a profile, the emoji picker.
- Destructive actions act at once and offer Undo in a toast
  (`c.ToastAction`), as Apple's guidelines suggest, rather than asking.
- Settings is its own window (⌘,), with tabs at the top and grouped rows of
  a label and a control, as System Settings.

### Keyboard and menus

Every Mac app has these, and so must this one (see `keys.go`):

| Menu | Items |
|---|---|
| App | About, 设置… ⌘,, Services, Hide ⌘H, Hide Others ⌥⌘H, Show All, Quit ⌘Q |
| 文件 | Close Window ⌘W (`RoleClose`) |
| Edit | the system's (`RoleEditMenu`): undo, redo, cut, copy, paste, select all |
| 显示 | 显示或隐藏边栏 ⌃⌘S; 放大 ⌘+, 缩小 ⌘−, 实际大小 ⌘0 (the text size); full screen ⌃⌘F |
| 前往 | 上一个会话 ⌥↑, 下一个会话 ⌥↓, 下一个未读会话 ⌥⇧↓ |
| Window | the system's (`RoleWindowMenu`): minimize ⌘M, zoom, bring all to front |

- ⌘W **hides** the main window rather than closing it, as Messages and
  Mail: the app stays connected and notifies, and a click on its Dock icon
  shows the same window again. Quitting (⌘Q) still closes it.
- Escape closes what is transient: a thread, a reply, an edit, a popover.
- Menu items run their work through `win.Update`, on the main thread.
- New commands go in a menu with their shortcut, never as a hidden key
  alone, so the menu bar teaches them.

### Files and pictures

- A click on a picture or a file shows it in **Quick Look**, as Messages
  does (`quicklook_darwin.go`, the system's `QLPreviewPanel`): zoom, full
  screen, Open with Preview, share, and the arrows between the pictures of
  the message. Never an in-app viewer of our own.
- Previewing never writes to the user's Downloads. Saving there is an
  explicit act: the card's button, or 存储到“下载” in the right-click menu,
  which also offers 快速查看, 在访达中显示 and 拷贝链接.
- What the app fetches lives in its **cache**, `~/Library/Caches/<bundle
  id>/`: `images/` for avatars and thumbnails (by a hash of the URL),
  `files/` for files previewed. It is trimmed to 500 MB, least recently used
  first, and 设置 › 通用 › 存储 shows its size and clears it. Nothing the app
  cannot fetch again goes there.

### Words

- The app is written in English and Simplified Chinese, and shows the one
  the user's languages prefer (System Settings › General › Language &
  Region, or the app's own language there), following a change at once.
  Every text the user reads is a message of the catalogs `locales/en.json`
  and `locales/zh-Hans.json`, said through the generated `locales` package
  (`locales.Send()`, `locales.Replies(n)`); never a string literal in the Go
  code. After changing a catalog, run `go tool mygo generate` and commit
  `locales/catalog_gen.go`. `import _ "github.com/egoist/mygo/locales/zh-Hans"`
  gives MyGo's own controls their Chinese; the menus AppKit labels (编辑,
  窗口, 关于, 退出) follow the app's language, so the menu bar never mixes
  languages. Use the system's standard menus and roles, and the system's
  names for the menus the app adds (文件, 显示, 前往). The tests read the app
  in Chinese (`locale_test.go`).
- Dates in the app's language through `i18n.FormatDate` (`date` in
  `main.go`), not `time.Format` with words in them.
- Simplified Chinese, in the words macOS uses: 拷贝 not 复制 for the
  clipboard, 存储 for saving an edit, 设置… with the ellipsis when it opens
  a window. English as macOS writes it: title case for menu items and
  buttons (Save to Downloads, Sign Out), sentence case for explanations.
- Keyboard shortcuts as macOS writes them: ⌘K, ⌃⌘S, ⌘,.

## Build target

Build and release **Apple Silicon only** (`darwin/arm64`). Do not build
Intel/x86_64 or universal binaries. CI and release artifacts are under
`build/darwin-arm64/`.

## Checking a change

- `go vet ./... && go tool mygo vet . && CGO_ENABLED=0 go test ./...`.
- Look at the real window, in light and dark: build with
  `go tool mygo build -platform darwin/arm64`, open the app and capture it with
  `screencapture -o -l <window id>`. Offscreen renders (`ui.Tester.Image`)
  show layout but neither the system's materials nor its menus.
- The README's screenshots come from a made-up workspace, never a real one.

## The app icon

`AppIcon.icon` is an Icon Composer icon with light and dark appearances; the
logo is drawn on the icon's own 1024-point canvas (its SVG is 1024 × 1024)
and placed at scale 1, which avoids guessing Icon Composer's scaling.
The canvas is the icon's rounded square: the logo's longer side spans 73%
of it (`translate(135 135) scale(5.937)`, 745 points), centered, as the
logos of Messages, Mail, Music, WeChat and Claude span 72-80% of theirs; at
half the canvas it looked small beside them. `mygo.json` points `macos.icon`
at the `.icon`, and `mygo build` compiles it with Xcode 26's `actool`: the
releases CI builds have it. A machine without `actool` skips it with a
message, and its build keeps `resources/icon.png`, the icon's background
in the 824-point rounded square of the icon grid with the logo drawn into
that square, as its icon (the sign-in card shows it too).
