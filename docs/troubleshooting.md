# Troubleshooting

Use SSH as the first verification path. If SSH works, the control connection, agent identity, and basic forwarding path are all in place.

## Check The Local Service First

Verify the target directly on the machine running `gotunnel`.

Examples:

```bash
ssh 127.0.0.1
```

```bash
curl http://127.0.0.1:3000/
```

If the local service is not healthy, the relay cannot fix that.

## Check The Agent

Run the agent in the foreground first if needed:

```bash
./gotunnel -config /path/to/agent.json
```

On macOS with `launchd`, inspect the service:

```bash
launchctl print "gui/$(id -u)/io.github.grepug.gotunnel.home-mac"
```

Common causes:

- wrong `auth_token`
- wrong `agent_id`
- wrong `relay_url`
- local target port is not listening

## Check The Relay

Inspect the relay status if `state_file` is configured:

```bash
./gotunneld -config /path/to/relay.json -status
```

Because `-status` does not coordinate with a live relay runtime, it renders persisted `active` records conservatively as offline `inactive` while preserving the last-known targets and connect timestamps.

## Check The Public Port

For HTTP-like targets:

```bash
curl http://your-vps:28080/
```

For SSH:

```bash
ssh -p 2222 your-user@your-vps
```

For desktop forwarding, use a client that matches the local service behind your `desktop` target.

## If The Tunnel Still Does Not Work

Work through the path in this order:

1. local service
2. agent process
3. relay process
4. public port mapping

If you are the relay operator, also re-check:

- the `ports[].agent_id` value
- the `ports[].target_name` value
- the agent target names in `agent.json`
- TLS file paths in relay config when using `wss://`
