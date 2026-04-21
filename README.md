# gotunnel

`gotunnel` is a personal self-hosted reverse tunnel for exposing selected local TCP services through a VPS.

Current `v1` focus:

- local SSH through a VPS
- generic TCP forwarding for local HTTP ports
- generic TCP forwarding for remote desktop ports

## Current shape

- `cmd/gotunneld`: relay process for the VPS
- `cmd/gotunnel`: local agent process
- one persistent control connection per local machine to the VPS
- fixed public TCP ports on the VPS
- token authentication
- explicit agent identity in static config
- automatic reconnect loop
- encrypted control plane by default

Plain `ws://` is only allowed when `allow_insecure` is explicitly set to `true`. That exists for local testing and development. Real deployments should use `wss://` with a valid certificate.

## Production TLS notes

For a real deployment, prefer `wss://` on the control plane and keep insecure mode limited to local development.

One workable production path is an IP certificate using current Certbot support for Let's Encrypt short-lived IP certificates. For a relay running directly on a public IP:

1. temporarily free inbound port `80` so ACME HTTP-01 can succeed
2. issue the certificate on the relay host with Certbot standalone
3. point `tls_cert_file` and `tls_key_file` at the issued files
4. restart `gotunneld`
5. switch the local agent config from `ws://.../connect` to `wss://.../connect`

Example issuance flow on Ubuntu:

```bash
sudo certbot certonly \
  --standalone \
  --non-interactive \
  --agree-tos \
  --register-unsafely-without-email \
  --preferred-profile shortlived \
  --ip-address 203.0.113.10
```

Typical relay config paths after issuance:

- `/etc/letsencrypt/live/<ip>/fullchain.pem`
- `/etc/letsencrypt/live/<ip>/privkey.pem`

Because these IP certificates are short-lived, add a Certbot deploy hook so the relay reloads after renewal.

Example deploy hook:

```bash
#!/bin/sh
systemctl restart gotunneld
```

Place it at:

```bash
/etc/letsencrypt/renewal-hooks/deploy/gotunneld-restart.sh
```

If the host already uses port `80` for another ingress layer, standalone renewal also needs a way to free that port temporarily. On the live Ubuntu deployment used for `gotunnel`, Certbot renewal is paired with pre/post hooks that pause and then restore the existing k3s ingress controller around renewal.

That pattern looks like:

- pre-hook: stop or unschedule the process currently bound to `:80`
- renewal: `certbot renew`
- post-hook: restore the ingress process after renewal completes

The important operational rule is simple: renewal must have a deterministic way to claim port `80`, or automatic renewal will silently fail later even if the first certificate issuance succeeded.

## Build

```bash
go build ./cmd/gotunnel ./cmd/gotunneld
```

## Quick start

1. Copy the example configs and replace the token and agent IDs.
2. For a real deployment, point the relay config at your TLS certificate and key and use a `wss://` relay URL in the agent config.
3. Start the relay on the VPS:

```bash
./gotunneld -config /path/to/relay.json
```

4. Start the agent on the local machine:

```bash
./gotunnel -config /path/to/agent.json
```

5. Connect to the VPS public port you mapped.

Example:

- relay port `2222` named `ssh` routed to `home-mac:ssh`
- local agent `home-mac`
- local target `ssh -> 127.0.0.1:22`

Then:

```bash
ssh -p 2222 your-user@your-vps
```

For local development only, you can set `allow_insecure: true` in both configs and use `ws://.../connect` instead of `wss://.../connect`.

## macOS launchd management

For a persistent local agent on macOS, prefer a per-user `LaunchAgent` over an ad hoc background shell process.

Template:

- [gotunnel.agent.plist.example](/Users/kai/Developer/utils/gotunnel/examples/gotunnel.agent.plist.example)

Recommended layout:

- binary: `~/.local/bin/gotunnel`
- config: `~/.config/gotunnel/<name>.json`
- plist: `~/Library/LaunchAgents/io.github.grepug.gotunnel.<name>.plist`
- logs: `~/Library/Logs/gotunnel/<name>.out.log` and `~/Library/Logs/gotunnel/<name>.err.log`

Basic management flow:

1. Copy the plist template and replace the user, config path, label suffix, and log paths.
2. Load it with `launchctl bootstrap`.
3. Use `launchctl print` and the log files for status.

Example commands:

```bash
mkdir -p ~/Library/LaunchAgents ~/Library/Logs/gotunnel
cp examples/gotunnel.agent.plist.example ~/Library/LaunchAgents/io.github.grepug.gotunnel.ssh-124-156-225-91.plist
```

Load:

```bash
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.ssh-124-156-225-91.plist
```

Restart after plist edits:

```bash
launchctl bootout "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.ssh-124-156-225-91.plist
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.ssh-124-156-225-91.plist
```

Status:

```bash
launchctl print "gui/$(id -u)/io.github.grepug.gotunnel.ssh-124-156-225-91"
```

Manual restart without editing the plist:

```bash
launchctl kickstart -k "gui/$(id -u)/io.github.grepug.gotunnel.ssh-124-156-225-91"
```

Unload:

```bash
launchctl bootout "gui/$(id -u)" ~/Library/LaunchAgents/io.github.grepug.gotunnel.ssh-124-156-225-91.plist
```

## Relay config

Example: [examples/relay.json](/Users/kai/Developer/utils/gotunnel/examples/relay.json)

Fields:

- `control_addr`: relay control listener
- `auth_tokens`: accepted agent tokens
- `tls_cert_file`: certificate for the control connection
- `tls_key_file`: private key for the control connection
- `allow_insecure`: only for local testing; allows plain `ws://`
- `ports`: public TCP listeners exposed on the VPS
- `ports[].name`: public listener name
- `ports[].agent_id`: which named agent should receive traffic for that listener
- `ports[].target_name`: which target on that agent should be opened

## Agent config

Example: [examples/agent.json](/Users/kai/Developer/utils/gotunnel/examples/agent.json)

Fields:

- `relay_url`: `wss://.../connect` in normal use
- `agent_id`: stable identity for this local machine or agent instance
- `auth_token`: shared token that must match the relay config
- `allow_insecure`: only for local testing with `ws://`
- `targets`: local services reachable through the tunnel

## Example flow

Example SSH mapping:

- relay port `0.0.0.0:2222` named `ssh` routed to agent `home-mac`
- agent `home-mac` target `ssh` mapped to `127.0.0.1:22`

With both binaries running, connecting to `vps:2222` reaches the local machine's SSH service through the tunnel.

Different agents can expose the same target name, such as `ssh`, as long as each relay port mapping points to the intended `agent_id` explicitly.

## Current limitations

- no hostname-based HTTP routing yet
- no session preservation across reconnect
- no multi-agent failover
- no persistent control-plane state yet
- no management API for dynamically registering or editing agents

Those are deliberate `A`-phase omissions so the transport core can stay small and reliable first.
