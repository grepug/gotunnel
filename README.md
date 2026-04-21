# gotunnel

`gotunnel` is a small self-hosted reverse tunnel for exposing selected local TCP services through a VPS.

Current `v1` focus:

- local SSH through a VPS
- generic TCP forwarding for local HTTP ports
- generic TCP forwarding for remote desktop ports

## Who it is for

`gotunnel` is for people who want a small tunnel they can run and understand themselves.

Good fit:

- reach your home or office machine over SSH through one VPS
- expose one or two local web ports on fixed public VPS ports
- forward a desktop port without setting up a full VPN

Not the goal:

- multi-tenant public tunnel SaaS
- hostname-based HTTP routing platform
- zero-config mesh VPN
- dynamic tunnel management UI or API

## Start Here

Start with three ideas:

1. One VPS runs `gotunneld`.
2. One local machine runs `gotunnel`.
3. Each public VPS port forwards to one named local target.

That is the whole system.

## What A User Needs

Most users only need these four values from the relay operator:

- `relay_url`
- `agent_id`
- `auth_token`
- the public ports that were assigned to that agent

Relay TLS and certificate files are operator-managed today. Public users should not need to edit `tls_cert_file` or `tls_key_file` just to run an agent.

## Build

`gotunnel` currently builds from source:

```bash
go build ./cmd/gotunnel ./cmd/gotunneld
```

## Quick Start

Start with SSH first. It is the smallest success path and the easiest to verify.

### VPS Side

Before the local machine does anything, the relay operator or VPS owner should already have:

- a running `gotunneld` relay
- one public SSH port mapped to your agent, such as `2222`
- your assigned `relay_url`
- your assigned `agent_id`
- your assigned `auth_token`

### Host Machine Side

### 1. Initialize local config

```bash
./gotunnel init
```

This creates the default local config home at `~/.gotunnel/agent.json`.

### 2. Save relay and auth settings

```bash
./gotunnel set relay --url wss://your-vps:18443/connect
./gotunnel set auth --agent-id home-mac --auth-token replace-me-home-mac
```

For local development only, you can use:

```bash
./gotunnel set relay --url ws://your-vps:18443/connect --allow-insecure
```

### 3. Add the SSH target

```bash
./gotunnel target add --name ssh --local-addr 127.0.0.1:22
```

You can inspect the stored config any time with:

```bash
./gotunnel show
```

### 4. Start the agent

```bash
./gotunnel run
```

### 5. Test SSH through the VPS

```bash
ssh -p 2222 your-user@your-vps
```

This example uses public SSH port `2222`. Use the public port that your relay operator assigned to your `ssh` target.

If that works, the tunnel is up.

## Add More Targets

Once SSH works, adding `web` and `desktop` is only more commands.

### VPS side

For each extra target, the relay operator or VPS owner still needs to map a public port to that target name on your agent.

### Add a web target

### Host machine side

```bash
./gotunnel target add --name web --local-addr 127.0.0.1:3000
```

Test:

```bash
curl http://your-vps:28080/
```

This example uses public web port `28080`. Use the public port assigned on your relay.

### Add a desktop target

### Host machine side

```bash
./gotunnel target add --name desktop --local-addr 127.0.0.1:5900
```

`desktop` is only a label. The relay does not care whether the local service behind that label is VNC, RDP, or another TCP desktop protocol.

Use the public desktop port assigned on your relay.

## For Relay Operators

If you run the VPS side, start here:

- [docs/relay-setup.md](docs/relay-setup.md)
- [docs/troubleshooting.md](docs/troubleshooting.md)

The operator guide covers:

- relay config and public port mappings
- TLS and certificate handling for `wss://`
- relay status inspection with `gotunneld -status`
- where `state_file` fits

Automatic TLS and certificate management are not built into `gotunnel` yet. Operators still manage those pieces outside the agent binary.

## Run The Agent On macOS

If you want the local agent to stay up through login sessions and restarts on macOS, use:

- [docs/macos-launchd.md](docs/macos-launchd.md)

## Examples

- relay example: [examples/relay.json](examples/relay.json)
- agent JSON example for advanced/manual use: [examples/agent.json](examples/agent.json)
- macOS launchd example: [examples/gotunnel.agent.plist.example](examples/gotunnel.agent.plist.example)

The shipped examples use one consistent naming scheme:

- `ssh`
- `web`
- `desktop`

## Current Shape

- `cmd/gotunneld`: relay process for the VPS
- `cmd/gotunnel`: local agent process
- one persistent control connection per local machine to the VPS
- fixed public TCP ports on the VPS
- token authentication
- explicit agent identity in static config
- automatic reconnect loop
- encrypted control plane by default

Plain `ws://` is only allowed when `allow_insecure` is explicitly set to `true`.
Manual JSON configs are still supported through `./gotunnel -config /path/to/agent.json` and `./gotunnel run -config /path/to/agent.json`.

## Troubleshooting

Start with these checks:

1. Verify the local target directly on the machine running `gotunnel`.
2. Check the agent logs for reconnect or authentication failures.
3. Check the relay logs on the VPS.
4. Run `gotunneld -status` on the VPS if the relay uses `state_file`.

Detailed operator troubleshooting lives in [docs/troubleshooting.md](docs/troubleshooting.md).

## Current Limitations

- no hostname-based HTTP routing yet
- no session preservation across reconnect
- no multi-agent failover
- no management API for dynamically registering or editing agents
- no dynamic agent provisioning yet

Those are deliberate `A`-phase omissions so the transport core can stay small first.
