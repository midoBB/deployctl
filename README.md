# deployctl

A CLI tool for managing containerized app deployments via [systemd quadlets](https://www.freedesktop.org/software/systemd/man/latest/systemd.container.html). Wraps `podman` and `systemctl` to provide atomic image updates, rollbacks, and health-verified deployments.

## Requirements

- Linux with systemd user services enabled
- Podman
- Quadlet container units configured under `~/.config/containers/systemd/` (or custom `--container-dir`)

## Installation

Download the latest binary from the [releases page](../../releases) and place it on your `$PATH`:

```sh
curl -Lo deployctl https://github.com/midoBB/deployctl/releases/latest/download/deployctl-linux-amd64
chmod +x deployctl
mv deployctl ~/.local/bin/
```

Or build from source:

```sh
make build
```

## Usage

```
deployctl [flags] <command> <app> [args]
```

### Commands

| Command | Description |
|---------|-------------|
| `deploy <app> <image>` | Deploy a new image and verify health |
| `rollback <app>` | Revert to the previously deployed image |
| `status <app>` | Show current image, service state, and health |
| `validate <app>` | Pre-flight checks before deployment |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--container-dir` | `~/.config/containers/systemd` | Directory containing quadlet `.container` files |
| `--json` | `false` | Output results as JSON |
| `--timeout` | `30s` | Timeout for health checks |

### Examples

```sh
# Deploy a new image
deployctl deploy myapp docker.io/myorg/myapp:v2.1.0

# Check status
deployctl status myapp

# Roll back if something goes wrong
deployctl rollback myapp

# Validate config before deploying
deployctl validate myapp
```

## Development

```sh
make test    # run tests
make build   # build binary to ./deployctl
make lint    # run go vet
```
