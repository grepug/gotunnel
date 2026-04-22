#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  bootstrap-relay-ubuntu.sh --binary <path-to-gotunneld> --ip <public-ip> --agent-id <agent-id> --auth-token <auth-token>

Optional flags:
  --control-port <port>   Default: 18443
  --ssh-port <port>       Default: 2222
  --web-port <port>       Default: 28080
  --desktop-port <port>   Default: 3389

This helper supports one path only:
  Ubuntu LTS + systemd + direct gotunneld TLS termination + Certbot short-lived IP certificates
EOF
}

BINARY=""
RELAY_IP=""
AGENT_ID=""
AUTH_TOKEN=""
CONTROL_PORT="18443"
SSH_PORT="2222"
WEB_PORT="28080"
DESKTOP_PORT="3389"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)
      BINARY="$2"
      shift 2
      ;;
    --ip)
      RELAY_IP="$2"
      shift 2
      ;;
    --agent-id)
      AGENT_ID="$2"
      shift 2
      ;;
    --auth-token)
      AUTH_TOKEN="$2"
      shift 2
      ;;
    --control-port)
      CONTROL_PORT="$2"
      shift 2
      ;;
    --ssh-port)
      SSH_PORT="$2"
      shift 2
      ;;
    --web-port)
      WEB_PORT="$2"
      shift 2
      ;;
    --desktop-port)
      DESKTOP_PORT="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$BINARY" || -z "$RELAY_IP" || -z "$AGENT_ID" || -z "$AUTH_TOKEN" ]]; then
  usage >&2
  exit 1
fi

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "This helper must run as root or through sudo." >&2
  exit 1
fi

if [[ ! -f "$BINARY" ]]; then
  echo "Binary not found: $BINARY" >&2
  exit 1
fi

BIN_DST="/opt/gotunnel/gotunneld"
CFG_DIR="/etc/gotunnel"
CFG_DST="${CFG_DIR}/relay.json"
STATE_DIR="/var/lib/gotunnel"
STATE_FILE="${STATE_DIR}/relay-state.json"
UNIT_DST="/etc/systemd/system/gotunneld.service"
CERT_DIR="/etc/letsencrypt/live/${RELAY_IP}"

install -d -m 0755 /opt/gotunnel
install -d -m 0755 "${CFG_DIR}"
install -d -m 0755 "${STATE_DIR}"

install -m 0755 "$BINARY" "$BIN_DST"

cat > "$CFG_DST" <<EOF
{
  "control_addr": "0.0.0.0:${CONTROL_PORT}",
  "agents": [
    {
      "agent_id": "${AGENT_ID}",
      "auth_token": "${AUTH_TOKEN}"
    }
  ],
  "state_file": "${STATE_FILE}",
  "tls_cert_file": "${CERT_DIR}/fullchain.pem",
  "tls_key_file": "${CERT_DIR}/privkey.pem",
  "ports": [
    {
      "name": "ssh",
      "listen_addr": "0.0.0.0:${SSH_PORT}",
      "agent_id": "${AGENT_ID}",
      "target_name": "ssh"
    },
    {
      "name": "web",
      "listen_addr": "0.0.0.0:${WEB_PORT}",
      "agent_id": "${AGENT_ID}",
      "target_name": "web"
    },
    {
      "name": "desktop",
      "listen_addr": "0.0.0.0:${DESKTOP_PORT}",
      "agent_id": "${AGENT_ID}",
      "target_name": "desktop"
    }
  ]
}
EOF

cat > "$UNIT_DST" <<'EOF'
[Unit]
Description=gotunnel relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/opt/gotunnel/gotunneld -config /etc/gotunnel/relay.json
WorkingDirectory=/opt/gotunnel
Restart=always
RestartSec=2
User=root
Group=root

[Install]
WantedBy=multi-user.target
EOF

echo
echo "Installed supported relay bootstrap assets:"
echo "  binary: ${BIN_DST}"
echo "  config: ${CFG_DST}"
echo "  state: ${STATE_FILE}"
echo "  unit:   ${UNIT_DST}"
echo
echo "Next: request the short-lived IP certificate with Certbot standalone."
echo
cat <<EOF
sudo certbot certonly \\
  --standalone \\
  --non-interactive \\
  --agree-tos \\
  --register-unsafely-without-email \\
  --preferred-profile shortlived \\
  --ip-address ${RELAY_IP}
EOF
echo
echo "Important:"
echo "  Certbot standalone must be able to claim port 80 during issuance and renewal."
echo "  If another process already uses port 80, stop it first or use a deterministic pre/post-hook flow."
echo
echo "Suggested deploy hook:"
echo
cat <<'EOF'
sudo install -d -m 0755 /etc/letsencrypt/renewal-hooks/deploy
sudo tee /etc/letsencrypt/renewal-hooks/deploy/gotunneld-restart.sh >/dev/null <<'HOOK'
#!/bin/sh
systemctl restart gotunneld
HOOK
sudo chmod 0755 /etc/letsencrypt/renewal-hooks/deploy/gotunneld-restart.sh
EOF
echo
echo "After certificate issuance:"
echo
cat <<'EOF'
sudo systemctl daemon-reload
sudo systemctl enable --now gotunneld
sudo systemctl status gotunneld
EOF
