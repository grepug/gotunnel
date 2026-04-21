---
name: gotunnel-agent
description: Use when setting up or operating the local gotunnel agent in this repo. Prefer the built-in gotunnel CLI to initialize config, set relay/auth values, add local targets, inspect stored config, and run the local tunnel from `~/.gotunnel/agent.json`. Do not use this skill for VPS relay bootstrap or raw TLS setup.
---

# Gotunnel Agent

Use this skill when the task is about the local machine side of `gotunnel`.

Examples:

- initialize local gotunnel config
- connect this machine to a relay with the assigned URL, agent ID, and auth token
- add or update `ssh`, `web`, or `desktop` targets
- inspect the stored local config
- run the local tunnel agent

Do not use this skill for:

- VPS or relay bootstrap
- Certbot, TLS, or relay-side port mapping work
- editing raw JSON unless the user explicitly asks for manual config or compatibility behavior

## Default Workflow

Prefer the CLI-first workflow:

```bash
./gotunnel init
./gotunnel set relay --url <relay-url>
./gotunnel set auth --agent-id <agent-id> --auth-token <auth-token>
./gotunnel target add --name <target-name> --local-addr <host:port>
./gotunnel show
./gotunnel run
```

Default config home:

- `~/.gotunnel/agent.json`

Use target names like:

- `ssh`
- `web`
- `desktop`

## Command Rules

- Prefer `./gotunnel run` when using the default config home.
- Use `./gotunnel show` before and after mutations when you need to confirm current state.
- Use `./gotunnel target add` as an upsert for existing target names.
- Use `--allow-insecure` only for local development with `ws://`.
- Preserve `./gotunnel -config /path/to/agent.json` and `./gotunnel run -config /path/to/agent.json` for compatibility cases or when the user explicitly wants a non-default path.

## Execution Guidance

- If `./gotunnel` is not present in the repo root yet, build it first with `go build ./cmd/gotunnel`.
- Treat the relay URL, agent ID, auth token, and public ports as operator-provided inputs.
- If the user asks how to make a local service reachable, add or update the local target on the host machine side first.
- If the user asks about macOS background execution, point them to `docs/macos-launchd.md` after the CLI config is initialized.
- If the task expands into relay bootstrap or TLS setup, switch to the relay/operator docs instead of extending this workflow.
