# gotunnel Agent CLI Design

## Goal

Replace manual agent JSON editing as the normal user workflow with a small built-in CLI.

## Scope

This slice is agent-side only.

It adds:

- a default config home at `~/.gotunnel/agent.json`
- commands to initialize, inspect, and update agent config
- a CLI-first README path

It does not add:

- relay management
- remote APIs
- dynamic provisioning
- config schema changes

## Supported workflow

Normal user flow:

1. `gotunnel init`
2. `gotunnel set relay --url ...`
3. `gotunnel set auth --agent-id ... --auth-token ...`
4. `gotunnel target add --name ssh --local-addr 127.0.0.1:22`
5. `gotunnel show`
6. `gotunnel run`

Compatibility flow:

- `gotunnel -config /path/to/agent.json`
- `gotunnel run -config /path/to/agent.json`

## Design

### Config storage

Keep `config.AgentConfig` as the only agent config schema.

Add helpers in `internal/config` to:

- resolve `~/.gotunnel/agent.json`
- load from that path
- save JSON to that path
- load/save explicit paths when `-config` is supplied

The CLI should create `~/.gotunnel` on first write.

### Command structure

Use standard-library argument parsing in `cmd/gotunnel`.

Commands:

- `init`
- `set relay`
- `set auth`
- `target add`
- `show`
- `run`

The old top-level `-config` invocation remains a compatibility shortcut for `run`.

### Mutations

`init`:

- writes a minimal config file if missing
- leaves an existing file intact unless an explicit overwrite flag is added later

`set relay`:

- updates `relay_url`

`set auth`:

- updates `agent_id`
- updates `auth_token`

`target add`:

- upserts by target name
- validates `local_addr`

### Output

`show` should print:

- config path
- relay URL
- agent ID
- targets as `name -> local_addr`

Mutating commands should print the config path they wrote.

### Backward compatibility

Manual JSON config files stay valid.

The runtime path still goes through:

- `config.LoadAgentConfig`
- `agent.NewClient`
- `client.Start`

## Testing

Add tests before implementation for:

- default config path resolution
- saving and loading default config
- CLI mutation commands
- compatibility `-config` run parsing

## Risks

- command parsing can sprawl if it tries to manage too many workflows
- CLI-generated config must not diverge from manual JSON config
- `run` must fail clearly when config is incomplete
