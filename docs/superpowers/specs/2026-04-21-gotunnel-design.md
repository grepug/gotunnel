# gotunnel v1 Design

## Status

Temporary local design record. This repo is not connected to a GitHub remote yet, so the preferred GitHub issue workflow cannot be used at this stage.

## Goal

Build a personal self-hosted tunnel in Go that makes selected local services reachable through a VPS.

Priority order:

1. Expose local SSH reliably
2. Expose local HTTP ports
3. Expose remote desktop ports

## Scope

### In scope for v1

- One VPS relay
- One local agent
- Personal single-user usage
- Fixed public TCP ports on the VPS
- Automatic reconnect after tunnel loss
- Shared-token authentication
- TCP forwarding for SSH, HTTP ports, and remote desktop ports

### Out of scope for v1

- Multi-user support
- Multi-agent high availability
- Hostname-based HTTP routing
- Preserving active sessions across tunnel reconnect
- Zero-downtime upgrades
- Multiple transport protocols

## Product Shape

The system has two executables:

- `gotunneld`: runs on the VPS and accepts public traffic
- `gotunnel`: runs on the local machine and connects outward to the VPS

The local machine keeps one secure outbound connection to the VPS. Public traffic accepted by the VPS is forwarded through that connection to configured local services.

This keeps the first version simple enough to make SSH useful quickly while preserving a clean path to a later multi-machine design.

## User Experience

The user configures:

- a VPS address
- a shared auth token
- which public VPS ports should map to which named local services
- which named local services map to which local addresses

Example:

- VPS public port `2222` forwards to named service `ssh`
- local named service `ssh` points to `127.0.0.1:22`

After startup:

- the agent connects to the VPS automatically
- the tunnel stays connected while healthy
- if the connection drops, the agent reconnects automatically
- when someone connects to the VPS public port, the traffic is forwarded to the configured local service

## Reliability Contract

v1 should be reliable enough for personal SSH use and acceptable for basic HTTP forwarding and occasional remote desktop usage.

v1 will explicitly support:

- agent reconnect after internet interruption
- relay restart recovery
- fast failure when the local target service is unavailable
- bounded memory behavior under slow or stalled connections

v1 will not promise:

- keeping active SSH or remote desktop sessions alive if the tunnel reconnects
- seamless failover between multiple local agents
- advanced routing behavior

## Security Model

v1 uses a shared token between the local agent and the VPS relay.

Security expectations:

- the tunnel connection is encrypted
- the relay accepts only configured tokens
- only configured public ports are exposed
- only configured local targets are reachable through the tunnel

This is acceptable for a personal first version. A later version can replace shared-token auth with per-agent identity without changing the core transport model.

## Configuration Model

Relay configuration should define:

- secure listen address
- TLS certificate and key
- accepted auth token list
- public port to service-name mappings

Agent configuration should define:

- relay address
- auth token
- service-name to local-address mappings

The configuration format should stay simple and static in v1. Persistent control-plane state is intentionally deferred to the later multi-machine version.

## Upgrade Path to Multi-Machine

The v1 boundaries should make the future transition to a broader personal multi-machine system straightforward.

Planned evolution path:

- replace the shared token with per-agent credentials
- replace purely static mappings with registered tunnel state
- support multiple named agents
- add more structured management and routing
- later add hostname-based HTTP routing on top of the existing forwarding core

The transport core should therefore stay generic and not bake HTTP-specific behavior into the first version.

## Implementation Boundaries

The work should be split into these major areas:

1. Shared configuration and protocol definitions
2. Relay process on the VPS
3. Local agent process
4. Connection lifecycle and reconnect behavior
5. TCP stream forwarding
6. Basic logging and verification tooling

This keeps the code small, testable, and ready for later growth without introducing control-plane complexity too early.

## Validation

The first implementation is complete when all of the following are true:

- the relay starts on the VPS with configured public ports
- the agent connects successfully from the local machine
- a connection to the VPS SSH port reaches the local SSH service
- a connection to a configured VPS HTTP port reaches the local HTTP service
- a connection to a configured VPS remote desktop port reaches the local remote desktop service
- tunnel loss triggers reconnect automatically
- failed local target dials fail fast instead of hanging indefinitely

## Reviewer Attention

- Connection lifecycle and reconnect behavior need close review because they define practical usability
- Stream forwarding must stay memory-safe under slow clients
- The auth and configuration boundary should stay simple now but cleanly replaceable later

## Next Step

After user review of this design, implementation should start with an SSH-first scaffold and tests around configuration, connection lifecycle expectations, and TCP forwarding behavior.
