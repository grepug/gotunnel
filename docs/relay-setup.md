# Relay Setup

This guide is for the person who runs the VPS relay.

If you only need to connect a local machine to an existing relay, go back to the [main README](../README.md) and use the agent quick start there.

If you want the first supported end-to-end operator path, start with [ubuntu-relay-bootstrap.md](ubuntu-relay-bootstrap.md).

## What The Relay Operator Owns

The relay operator manages:

- the VPS
- the `gotunneld` process
- relay TLS for `wss://`
- agent credentials
- public port assignments
- the optional relay `state_file`

Automatic TLS and certificate management are not built into `gotunnel` yet. If you want encrypted control connections, the operator must provide the certificate and key paths in relay config.

## Build

```bash
go build ./cmd/gotunnel ./cmd/gotunneld
```

## Supported Bootstrap Path

For `v1`, the first supported operator path is:

- one public Ubuntu LTS VPS
- `systemd`
- direct `gotunneld` TLS termination
- Certbot-managed short-lived IP certificates

Use:

- [ubuntu-relay-bootstrap.md](ubuntu-relay-bootstrap.md)
- [../examples/relay.ubuntu-ip.json](../examples/relay.ubuntu-ip.json)
- [../examples/gotunneld.service.example](../examples/gotunneld.service.example)
- [../scripts/bootstrap-relay-ubuntu.sh](../scripts/bootstrap-relay-ubuntu.sh)

## Relay Config

Example:

- [../examples/relay.json](../examples/relay.json)

Example relay config:

```json
{
  "control_addr": "0.0.0.0:18443",
  "agents": [
    {
      "agent_id": "home-mac",
      "auth_token": "replace-me-home-mac"
    }
  ],
  "state_file": "/var/lib/gotunnel/relay-state.json",
  "tls_cert_file": "/etc/letsencrypt/live/example.com/fullchain.pem",
  "tls_key_file": "/etc/letsencrypt/live/example.com/privkey.pem",
  "ports": [
    {
      "name": "ssh",
      "listen_addr": "0.0.0.0:2222",
      "agent_id": "home-mac",
      "target_name": "ssh"
    },
    {
      "name": "web",
      "listen_addr": "0.0.0.0:28080",
      "agent_id": "home-mac",
      "target_name": "web"
    },
    {
      "name": "desktop",
      "listen_addr": "0.0.0.0:3389",
      "agent_id": "home-mac",
      "target_name": "desktop"
    }
  ]
}
```

## Field Summary

- `control_addr`: relay control listener
- `agents`: relay-side credentials for each named agent
- `agents[].agent_id`: the only agent name allowed to use that credential
- `agents[].auth_token`: the shared secret for that agent
- `state_file`: optional relay-local JSON file for persisted last-known registration state
- `tls_cert_file`: certificate for the control connection
- `tls_key_file`: private key for the control connection
- `allow_insecure`: local testing only; allows plain `ws://`
- `ports`: public TCP listeners exposed on the VPS
- `ports[].agent_id`: which named agent should receive traffic
- `ports[].target_name`: which target on that agent should be opened

## Start The Relay

```bash
./gotunneld -config /path/to/relay.json
```

## TLS Notes

For real deployments, prefer `wss://` on the control plane.

Today that means the operator must provide:

- a certificate file path in `tls_cert_file`
- a key file path in `tls_key_file`

If you are testing locally, you can set `allow_insecure: true` and use `ws://.../connect`. Do not treat that as a normal public deployment mode.

## First Operator Verification

1. Start the relay.
2. Start the agent on the local machine.
3. Test SSH first with `ssh -p 2222 your-user@your-vps`.
4. Then test `web` and `desktop` if those public ports were configured.

## Inspect Relay Registration Status

If `state_file` is configured, `gotunneld` can print the persisted named-agent registration state without starting the relay server:

```bash
./gotunneld -config /path/to/relay.json -status
```

Example output shape:

```text
home-mac	inactive	targets=ssh,web,desktop	last_connected=2026-04-21T13:00:00Z	last_disconnected=-
lab-mini	never_connected	targets=-	last_connected=-	last_disconnected=-
```

This command is read-only. It prints the persisted last-known registration state from the relay-owned state file and does not create, edit, or delete relay registrations, credentials, or port mappings.

Because `-status` does not coordinate with a live relay runtime, it renders persisted `active` records conservatively as offline `inactive` while preserving the last-known targets and connect timestamps.

## Related Docs

- agent onboarding: [../README.md](../README.md)
- macOS agent persistence: [macos-launchd.md](macos-launchd.md)
- troubleshooting: [troubleshooting.md](troubleshooting.md)
