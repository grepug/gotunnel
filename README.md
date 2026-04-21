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

### 1. Create an agent config

Example:

- [examples/agent.json](examples/agent.json)

Minimal agent config:

```json
{
  "relay_url": "wss://your-vps:18443/connect",
  "agent_id": "home-mac",
  "auth_token": "replace-me-home-mac",
  "targets": [
    {
      "name": "ssh",
      "local_addr": "127.0.0.1:22"
    }
  ]
}
```

### 2. Start the agent

```bash
./gotunnel -config /path/to/agent.json
```

### 3. Test SSH through the VPS

```bash
ssh -p 2222 your-user@your-vps
```

This example uses public SSH port `2222`. Use the public port that your relay operator assigned to your `ssh` target.

If that works, the tunnel is up.

For local development only, you can set `allow_insecure: true` and use `ws://.../connect` instead of `wss://.../connect`.

## Add More Targets

Once SSH works, adding `web` and `desktop` is only more config.

### Add a web target

```json
{
  "name": "web",
  "local_addr": "127.0.0.1:3000"
}
```

Test:

```bash
curl http://your-vps:28080/
```

This example uses public web port `28080`. Use the public port assigned on your relay.

### Add a desktop target

```json
{
  "name": "desktop",
  "local_addr": "127.0.0.1:5900"
}
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
- agent example: [examples/agent.json](examples/agent.json)
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
