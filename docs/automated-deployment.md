# Automated Container Deployment with GitHub Actions and deployctl

This guide covers implementing automated deployment of containerized applications using `deployctl`, a CLI tool for managing systemd quadlet-based container deployments. The workflow covers building containers, pushing to a private registry, and automatically redeploying services via SSH over Tailscale.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Quadlet Container Setup](#quadlet-container-setup)
- [GitHub Actions Setup](#github-actions-setup)
- [Complete Deployment Workflow](#complete-deployment-workflow)
- [Container Management](#container-management)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)

## Overview

This automated deployment system enables:

- **Continuous Deployment** — Automatic deployment when code is merged to main
- **Health-verified updates** — `deployctl` waits for your container's health check before marking a deployment successful
- **Automatic rollback** — Revert to the last known-good image with a single command
- **Secure communication** — Tailscale for SSH access from GitHub Actions to your server
- **Private registry** — Zot OCI registry for container image storage

### Workflow Summary

```
Code Push → GitHub Actions → Build & Push Image → SSH via Tailscale → deployctl deploy
                                                                             │
                                                         ┌───────────────────┤
                                                         ▼                   │
                                                   Health Check OK?    No → Rollback
                                                         │
                                                         ▼
                                                   Deployment complete
```

## Architecture

### Components

- **deployctl** — CLI that manages the deploy/rollback/status lifecycle on your server
- **systemd quadlets** — Declarative container unit files under `~/.config/containers/systemd/`
- **Podman** — Rootless container runtime (user-scope systemd, no root required)
- **Zot Registry** — Private OCI registry for container images
- **Tailscale** — Secure network for SSH from GitHub Actions to your server
- **GitHub Actions** — CI/CD pipeline

### How deployctl fits in

`deployctl` runs directly on your server. It edits the `Image=` line in your quadlet `.container` file, reloads systemd, restarts the service, and waits for the health check to pass — all atomically. On failure it restores the previous image. GitHub Actions triggers this over SSH.

## Prerequisites

### Server requirements

- Linux with systemd user services enabled (`loginctl enable-linger $USER`)
- Podman installed and working rootless
- `deployctl` installed:
  ```sh
  curl -Lo deployctl https://github.com/midoBB/deployctl/releases/latest/download/deployctl-linux-amd64
  chmod +x deployctl
  mv deployctl ~/.local/bin/
  ```
- Tailscale installed and connected
- A Zot registry accessible from the server

### Tailscale setup for GitHub Actions

1. Go to [Tailscale Admin Console](https://login.tailscale.com/admin) → Settings → OAuth clients
2. Create a client with `devices:write` scope — save the Client ID and Client Secret
3. Add a CI tag to your ACL if not already present:
   ```json
   {
     "tagOwners": {
       "tag:ci": []
     }
   }
   ```

### GitHub repository secrets

```
REGISTRY_URL          registry.your-domain.com
REGISTRY_USERNAME     your-registry-username
REGISTRY_PASSWORD     your-registry-password
TS_OAUTH_CLIENT_ID    tskey-client-xxx
TS_OAUTH_SECRET       tskey-xxx
DEPLOY_HOST           your-server-tailscale-hostname
DEPLOY_SSH_KEY        private SSH key for the deploy user
```

## Quadlet Container Setup

Each application is described by a `.container` quadlet file. `deployctl` reads and updates these files on deploy.

### Example: `~/.config/containers/systemd/myapp.container`

```ini
[Unit]
Description=My Application
After=network-online.target

[Container]
Image=registry.your-domain.com/my-org/myapp:latest
ContainerName=myapp
PublishPort=3000:3000
EnvironmentFile=%h/.config/myapp/env

[Service]
Restart=always

[Install]
WantedBy=default.target
```

Key points:
- `Image=` is the field `deployctl` updates on each deploy
- `ContainerName=` is used to query container health status
- Add a `HealthCmd=` under `[Container]` for health-verified deploys (strongly recommended)

### Adding a health check

```ini
[Container]
Image=registry.your-domain.com/my-org/myapp:latest
ContainerName=myapp
PublishPort=3000:3000
HealthCmd=curl -sf http://localhost:3000/health
HealthInterval=5s
HealthTimeout=3s
HealthRetries=3
```

With a health check defined, `deployctl deploy` won't return until the container reports `healthy` (or times out and rolls back).

### Enabling the service

```sh
systemctl --user daemon-reload
systemctl --user enable --now myapp
```

### State files

`deployctl` writes state to `~/.local/state/containers/systemd/myapp.state.json`, tracking the current image, previous image, and last-known-good image for rollbacks.

## GitHub Actions Setup

### Complete deployment workflow

Create `.github/workflows/deploy.yml`:

```yaml
name: Build, Push, and Deploy

on:
  push:
    branches: [main]
    tags: ['v*']

env:
  REGISTRY: ${{ secrets.REGISTRY_URL }}
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    outputs:
      image-ref: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.sha }}

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ secrets.REGISTRY_USERNAME }}
          password: ${{ secrets.REGISTRY_PASSWORD }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.sha }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max

  deploy:
    needs: build-and-push
    runs-on: ubuntu-latest

    steps:
      - name: Configure SSH
        run: |
          mkdir -p ~/.ssh
          echo "${{ secrets.DEPLOY_SSH_KEY }}" > ~/.ssh/id_ed25519
          chmod 600 ~/.ssh/id_ed25519

      - name: Connect to Tailscale
        uses: tailscale/github-action@v2
        with:
          oauth-client-id: ${{ secrets.TS_OAUTH_CLIENT_ID }}
          oauth-secret: ${{ secrets.TS_OAUTH_SECRET }}
          tags: tag:ci

      - name: Deploy via deployctl
        run: |
          ssh -o StrictHostKeyChecking=no deploy@${{ secrets.DEPLOY_HOST }} \
            "deployctl deploy myapp ${{ needs.build-and-push.outputs.image-ref }}"

      - name: Check deployment status on failure
        if: failure()
        run: |
          ssh deploy@${{ secrets.DEPLOY_HOST }} "deployctl status myapp --json"
```

### Setting up SSH key authentication

Generate a dedicated deploy key:

```sh
ssh-keygen -t ed25519 -C "github-actions-deploy" -f deploy_key -N ""
```

Add the public key to `~/.ssh/authorized_keys` on your server, and store the private key as the `DEPLOY_SSH_KEY` secret in GitHub.

## Complete Deployment Workflow

### 1. Validate before deploying (optional but recommended)

```sh
deployctl validate myapp
```

Checks performed:
- Container file exists and is readable
- Image reference is parseable
- Image can be pulled from the registry
- systemd unit is loaded
- Network dependencies are active
- All environment files are readable

### 2. Deploy

```sh
deployctl deploy myapp registry.your-domain.com/my-org/myapp:v2.1.0
```

`deployctl` will:
1. Parse the quadlet file
2. Save the current image as a rollback target
3. Update `Image=` atomically
4. Run `systemctl --user daemon-reload && systemctl --user restart myapp`
5. Poll for `active/running` state and (if configured) `healthy` container status
6. Print success or restore the previous image on failure

Output (text):
```
app=myapp deploy=ok image=registry.your-domain.com/my-org/myapp:v2.1.0
```

Output (`--json`):
```json
{"app":"myapp","deploy":"ok","image":"registry.your-domain.com/my-org/myapp:v2.1.0"}
```

### 3. Check status

```sh
deployctl status myapp
```

Shows the current image, service state, and container health.

### 4. Roll back if needed

```sh
deployctl rollback myapp
```

Restores the last known-good image (falling back to previous image if needed), then restarts and health-checks the service.

## Container Management

### Flags reference

```sh
deployctl deploy myapp <image> [--timeout 120s] [--dry-run] [--json]
deployctl rollback myapp         [--timeout 60s]  [--dry-run] [--json]
deployctl status myapp                                         [--json]
deployctl validate myapp                                       [--json]
```

| Flag | Commands | Default | Description |
|------|----------|---------|-------------|
| `--timeout` | deploy, rollback | 60s | Health check timeout |
| `--dry-run` | deploy, rollback | false | Preview changes without applying |
| `--json` | all | false | JSON output |
| `--container-dir` | all | `~/.config/containers/systemd` | Quadlet directory |

### Dry-run before deploying

```sh
deployctl deploy myapp registry.your-domain.com/my-org/myapp:v2.1.0 --dry-run
```

Shows what would happen without touching the service.

## Security Considerations

### SSH hardening for the deploy user

Restrict the deploy key to only run `deployctl`:

```
# ~/.ssh/authorized_keys
command="deployctl deploy myapp $SSH_ORIGINAL_COMMAND",no-port-forwarding,no-X11-forwarding,no-agent-forwarding ssh-ed25519 AAAA...
```

### Tailscale ACL

Restrict the `tag:ci` device to only reach your server's SSH port:

```json
{
  "acls": [
    {
      "action": "accept",
      "src": ["tag:ci"],
      "dst": ["tag:server:22"]
    }
  ]
}
```

### Rootless containers

`deployctl` uses `systemctl --user` and Podman in rootless mode — no root access required on the server. Do not run `deployctl` as root.

### Registry authentication

Store registry credentials in a Podman auth file rather than environment variables:

```sh
podman login registry.your-domain.com --authfile ~/.config/containers/auth.json
```

## Troubleshooting

### View deployment state

```sh
# Current status
deployctl status myapp

# Raw state file
cat ~/.local/state/containers/systemd/myapp.state.json
```

### Inspect the service

```sh
# Service status
systemctl --user status myapp

# Live logs (deployctl tails these automatically on failure)
journalctl --user -u myapp -f

# Container health
podman inspect myapp --format '{{.State.Health.Status}}'
```

### Deployment failed — service didn't start

```sh
# Check if the image and config are valid
deployctl validate myapp

# Check what went wrong
journalctl --user -u myapp -n 50

# Roll back immediately
deployctl rollback myapp
```

### Image pull failed

```sh
# Test registry auth
podman pull registry.your-domain.com/my-org/myapp:v2.1.0

# Check image is accessible
podman manifest inspect registry.your-domain.com/my-org/myapp:v2.1.0
```

### Tailscale connection issues in Actions

```yaml
- name: Verify Tailscale
  run: |
    tailscale status
    tailscale ping ${{ secrets.DEPLOY_HOST }}
```

### Health check timing out

Increase the timeout for slow-starting applications:

```sh
deployctl deploy myapp <image> --timeout 120s
```

Or test the health check directly on the server:

```sh
podman healthcheck run myapp
```
