# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Sideterm

Sideterm is a small Go daemon that launches a kitty terminal with remote control enabled and listens to i3 window manager events to automatically manage kitty tabs for Emacs projects. When an Emacs window title changes (format: `projectName - projectPath`), sideterm either focuses an existing kitty tab for that project or creates a new one with a splits layout via the kitty remote control protocol over a unix socket at `$HOME/tmp/emacs-kitty`.

## Build & Run

```bash
go build -o sideterm .
```

There are no tests or linter configured yet.

## Architecture

Two files, single `main` package:

- **main.go** — Subscribes to i3 `WindowEvent`s, filters for Emacs title changes matching `^(.+) - (.+)$`, extracts project name/path, and delegates to `handleProject` which looks up or creates kitty tabs.
- **kitty.go** — Kitty remote control client over unix socket. Sends DCS-framed JSON commands (`\x1bP@kitty-cmd{...}\x1b\\`) and parses DCS-framed responses. Exposes `listTabs`, `focusTab`, and `createProjectTab` (creates a tab + horizontal split, both with cwd set to the project path).

## Dependencies

- `go.i3wm.org/i3/v4` — i3 IPC library for subscribing to window events.
