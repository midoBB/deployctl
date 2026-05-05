# Blue/Green Deployments with Caddy Integration

This document defines a plan for extending `deployctl` to support **zero-downtime deployments** using a **blue/green (dual-slot) model** with **Caddy as the traffic switch**.

The goal is to evolve from **in-place restarts** to **traffic-controlled releases with instant rollback**.

---

## Objectives

- Run **two versions of an app simultaneously** (blue/green)
- Deploy to the **inactive slot**
- Verify health **before serving traffic**
- Switch traffic via **Caddy reload**
- Enable **instant rollback without container restart**
- Preserve existing `deployctl` behavior and UX

---

## High-Level Architecture

```

```

            ┌──────────────┐
            │    Caddy     │
            │ reverse_proxy│
            └──────┬───────┘
                   │
    ┌──────────────┴──────────────┐
    │                             │

```

active slot                  inactive slot
(myapp-blue)                  (myapp-green)
:18080                         :18081

```

- Only one slot receives traffic at a time
- Both slots can run concurrently
- Caddy determines which slot is “live”

---

## Required Changes

### 1. Dual Quadlet Units

Each app must have **two quadlet files**:

```

~/.config/containers/systemd/
├── myapp-blue.container
└── myapp-green.container

```

Example:

```ini
# myapp-blue.container
[Container]
ContainerName=myapp-blue
Image=...
PublishPort=127.0.0.1:18080:3000
HealthCmd=curl -sf http://localhost:3000/health
```

```ini
# myapp-green.container
[Container]
ContainerName=myapp-green
Image=...
PublishPort=127.0.0.1:18081:3000
HealthCmd=curl -sf http://localhost:3000/health
```

Constraints:

- Ports must differ
- Health checks must be defined

---

### 2. Caddy Upstream Indirection

Replace static upstreams with a **generated or managed config**.

#### Recommended: file-based include

Caddyfile:

```caddy
example.com {
    import upstreams/myapp.conf
}
```

Generated file:

```
/etc/caddy/upstreams/myapp.conf
```

Contents:

```
reverse_proxy 127.0.0.1:18080
```

`deployctl` will update this file and run:

```sh
caddy reload
```

---

### 3. State Model Update

Extend existing state file:

```
~/.local/state/containers/systemd/myapp.state.json
```

Add:

```json
{
  "active_slot": "blue",
  "slots": {
    "blue": {
      "image": "...",
      "port": 18080
    },
    "green": {
      "image": "...",
      "port": 18081
    }
  }
}
```

---

## Deployment Flow (New)

Replace current in-place restart with:

```text
1. Determine active slot
2. Select inactive slot
3. Update quadlet (inactive) with new image
4. Restart inactive unit
5. Wait for health check
6. If healthy:
     - Update Caddy upstream → inactive port
     - Reload Caddy
     - Mark slot active
   Else:
     - Stop inactive
     - Keep current active unchanged
```

---

## Rollback Flow (New)

```text
1. Read previous active slot
2. Update Caddy upstream → previous slot port
3. Reload Caddy
4. Mark previous slot active
```

Notes:

- No container restart required
- Rollback is near-instant

---

## CLI Behavior Changes

### `deploy`

- Now performs **blue/green deployment**
- No downtime expected
- Still respects `--timeout`

### `rollback`

- Switches traffic only (fast path)
- Optional: fallback to restart if slot missing

### `status`

Extend output:

```
app=myapp
active=blue
blue=image@sha256:...
green=image@sha256:...
proxy=127.0.0.1:18080
```

---

## New Internal Functions

### Slot Resolution

```go
func getActiveSlot(app string) string
func getInactiveSlot(active string) string
```

---

### Proxy Control

```go
func writeCaddyUpstream(app string, port int) error
func reloadCaddy() error
```

Implementation:

- overwrite upstream file
- call `caddy reload`

---

### Health Check (existing, reused)

```go
waitForHealthy(containerName string, timeout time.Duration)
```

---

## Backward Compatibility

Support both modes:

### Legacy mode (default)

- Single quadlet
- Current behavior unchanged

### Blue/Green mode (auto-detected)

- If `*-blue.container` and `*-green.container` exist
- Switch to new deployment logic

---

## Failure Handling

### If new slot fails health check:

- Stop new container
- Do not touch Caddy
- Return error

### If Caddy reload fails:

- Keep old slot active
- Report failure
- Do not mark deployment successful

---

## Security Considerations

- Ensure deploy user can run:
  - `systemctl --user`
  - `caddy reload`

- Avoid arbitrary file writes (restrict upstream path)
- Validate ports before writing config

---

## Future Extensions (Optional)

- Weighted traffic (canary releases)
- Automatic rollback on runtime failure
- Multi-instance scaling per slot
- Metrics-based promotion

---

## Summary

This change shifts deployment strategy from:

```
restart container → verify
```

to:

```
start new → verify → switch traffic
```

Result:

- Zero downtime deployments
- Instant rollback
- Minimal changes to existing architecture
- No need for full orchestrators (Nomad/Kubernetes)
