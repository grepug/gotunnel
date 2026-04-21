# macOS launchd

This guide is for running `gotunnel` persistently on macOS with a per-user `LaunchAgent`.

Template:

- [../examples/gotunnel.agent.plist.example](../examples/gotunnel.agent.plist.example)

Recommended layout:

- binary: `~/.local/bin/gotunnel`
- config: `~/.config/gotunnel/<name>.json`
- plist: `~/Library/LaunchAgents/io.github.grepug.gotunnel.<name>.plist`
- logs: `~/Library/Logs/gotunnel/<name>.out.log` and `~/Library/Logs/gotunnel/<name>.err.log`

## Basic Flow

1. Copy the plist template.
2. Replace the user path, config path, label suffix, and log paths.
3. Load it with `launchctl bootstrap`.
4. Use `launchctl print` and the log files for status.

Example commands:

```bash
mkdir -p ~/Library/LaunchAgents ~/Library/Logs/gotunnel
cp examples/gotunnel.agent.plist.example ~/Library/LaunchAgents/io.github.grepug.gotunnel.home-mac.plist
```

Load:

```bash
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.home-mac.plist
```

Restart after plist edits:

```bash
launchctl bootout "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.home-mac.plist
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.home-mac.plist
```

Status:

```bash
launchctl print "gui/$(id -u)/io.github.grepug.gotunnel.home-mac"
```

Manual restart without editing the plist:

```bash
launchctl kickstart -k "gui/$(id -u)/io.github.grepug.gotunnel.home-mac"
```

Unload:

```bash
launchctl bootout "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.home-mac.plist
```

If you only need one-off testing, you do not need `launchd`. Running `./gotunnel -config /path/to/agent.json` in a shell is enough.
