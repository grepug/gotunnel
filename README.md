# gotunnel

`gotunnel` is a personal self-hosted reverse tunnel for exposing selected local TCP services through a VPS.

Current `v1` focus:

- local SSH through a VPS
- generic TCP forwarding for local HTTP ports
- generic TCP forwarding for remote desktop ports

## Current shape

- `cmd/gotunneld`: relay process for the VPS
- `cmd/gotunnel`: local agent process
- one persistent control connection from the local machine to the VPS
- fixed public TCP ports on the VPS
- token authentication
- automatic reconnect loop
- encrypted control plane by default

Plain `ws://` is only allowed when `allow_insecure` is explicitly set to `true`. That exists for local testing and development. Real deployments should use `wss://` with a valid certificate.

## Build

```bash
go build ./cmd/gotunnel ./cmd/gotunneld
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

## Agent config

Example: [examples/agent.json](/Users/kai/Developer/utils/gotunnel/examples/agent.json)

Fields:

- `relay_url`: `wss://.../connect` in normal use
- `auth_token`: shared token that must match the relay config
- `allow_insecure`: only for local testing with `ws://`
- `targets`: local services reachable through the tunnel

## Example flow

Example SSH mapping:

- relay port `0.0.0.0:2222` named `ssh`
- agent target `ssh` mapped to `127.0.0.1:22`

With both binaries running, connecting to `vps:2222` reaches the local machine's SSH service through the tunnel.

## Current limitations

- no hostname-based HTTP routing yet
- no session preservation across reconnect
- no multi-agent failover
- no persistent control-plane state yet

Those are deliberate `A`-phase omissions so the transport core can stay small and reliable first.
