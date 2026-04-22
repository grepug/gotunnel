# Ubuntu Relay Bootstrap

This guide is the first supported relay-operator path for `gotunnel v1`.

Supported scope:

- one public Ubuntu LTS VPS
- `systemd` for service management
- direct `gotunneld` TLS termination
- Certbot-managed short-lived IP certificates
- one relay binary at `/opt/gotunnel/gotunneld`
- one relay config at `/etc/gotunnel/relay.json`
- one relay state file at `/var/lib/gotunnel/relay-state.json`

Out of scope for this guide:

- domain-based TLS variants
- reverse-proxy-managed TLS
- non-`systemd` Linux setups
- multi-node relay topologies

## What You Need

Before you start, have:

- a public Ubuntu VPS
- root or sudo access
- a built `gotunneld` binary available locally or on the host
- the public IP you want to use for the relay
- at least one `agent_id` and `auth_token` pair for the first local machine

## Repo Assets

This repo now ships the supported bootstrap assets:

- bootstrap helper: [../scripts/bootstrap-relay-ubuntu.sh](../scripts/bootstrap-relay-ubuntu.sh)
- supported relay config template: [../examples/relay.ubuntu-ip.json](../examples/relay.ubuntu-ip.json)
- `systemd` unit example: [../examples/gotunneld.service.example](../examples/gotunneld.service.example)

## Fastest Path

If you already have a `gotunneld` binary on the Ubuntu host, run the helper script there:

```bash
sudo bash ./scripts/bootstrap-relay-ubuntu.sh \
  --binary ./gotunneld \
  --ip 203.0.113.10 \
  --agent-id home-mac \
  --auth-token replace-me-home-mac
```

That script does three things:

1. installs the relay binary to `/opt/gotunnel/gotunneld`
2. writes `/etc/gotunnel/relay.json`
3. installs `/etc/systemd/system/gotunneld.service`

It does not request certificates for you. Instead, it prints the exact Certbot issuance and renewal commands for the supported path.

## Installed Layout

After the helper runs, the supported layout is:

- binary: `/opt/gotunnel/gotunneld`
- config: `/etc/gotunnel/relay.json`
- state: `/var/lib/gotunnel/relay-state.json`
- systemd unit: `/etc/systemd/system/gotunneld.service`
- TLS cert: `/etc/letsencrypt/live/<relay-ip>/fullchain.pem`
- TLS key: `/etc/letsencrypt/live/<relay-ip>/privkey.pem`

## Certificate Issuance

For the supported path, use Certbot standalone with a short-lived IP certificate.

Example:

```bash
sudo certbot certonly \
  --standalone \
  --non-interactive \
  --agree-tos \
  --register-unsafely-without-email \
  --preferred-profile shortlived \
  --ip-address 203.0.113.10
```

Important constraint:

- Certbot standalone must be able to claim port `80`

If another process already uses port `80`, stop it first or add a deterministic pre/post-hook flow for renewal.

## Renewal And Restart Contract

The supported renewal contract is:

1. Certbot renews the short-lived IP certificate.
2. A Certbot deploy hook restarts `gotunneld`.
3. If another process uses port `80`, a pre-hook frees it and a post-hook restores it.

Example deploy hook:

```bash
#!/bin/sh
systemctl restart gotunneld
```

Suggested path:

```bash
/etc/letsencrypt/renewal-hooks/deploy/gotunneld-restart.sh
```

The helper script prints the exact commands for creating this hook.

## Start The Relay

After certificate issuance, start and enable the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gotunneld
```

Check status:

```bash
sudo systemctl status gotunneld
```

## Verify The Relay

Inspect the config:

```bash
sudo cat /etc/gotunnel/relay.json
```

Inspect persisted registration state when needed:

```bash
sudo /opt/gotunnel/gotunneld -config /etc/gotunnel/relay.json -status
```

Then connect a local machine using the assigned:

- `relay_url`
- `agent_id`
- `auth_token`

## Related Docs

- relay overview: [relay-setup.md](relay-setup.md)
- troubleshooting: [troubleshooting.md](troubleshooting.md)
- agent onboarding: [../README.md](../README.md)
