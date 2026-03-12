![beepboop header](images/beepboop-repo.png)

[![CI](https://github.com/jordantrizz/beepboop/actions/workflows/ci.yml/badge.svg)](https://github.com/jordantrizz/beepboop/actions/workflows/ci.yml)
[![Release Workflow](https://github.com/jordantrizz/beepboop/actions/workflows/release.yml/badge.svg)](https://github.com/jordantrizz/beepboop/actions/workflows/release.yml)
[![Latest Release](https://img.shields.io/github/v/release/jordantrizz/beepboop?display_name=tag&sort=semver)](https://github.com/jordantrizz/beepboop/releases/latest)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)](https://github.com/jordantrizz/beepboop/blob/main/go.mod)


# beepboop

**beepboop** is a cross-platform terminal command that checks a host or website and emits a terminal beep when the target is up.

# Donate
If you like any of the scripts or tools, please consider donating to help support the development of these tools.

[![Buy Me A Coffee](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://ko-fi.com/jordantrask)
[![ManagingWP](https://i.imgur.com/x5SjITX.png)](https://managingwp.io/sponsor)

## Features
- Checks host/IP reachability with `icmp`
- Checks websites with `http`/`https`
- Checks TCP/UDP port connectivity with `tcp`/`udp`
- Auto-detects host vs URL when `--mode auto` (default)
- **Multiple checks** — require ICMP + TCP ports + HTTP/HTTPS all to pass before signalling up (see `--checks`)
- Follows HTTP redirects by default (bounded)
- Runs until success by default (use `--once` for a single check)
- Colorized terminal status output (with plain-text fallback)
- Works across Linux, macOS, and Windows via Go cross-compilation

## Installation

### Option 1: Download a release binary (recommended)

- Open the latest release assets in GitHub Releases.
- Download the archive for your OS/architecture.
- Extract and run `beepboop` (`beepboop.exe` on Windows).

#### Platform install examples

Linux/macOS (`tar.gz`):

```bash
curl -L -o beepboop.tar.gz <RELEASE_ASSET_URL>
tar -xzf beepboop.tar.gz
chmod +x beepboop-*
sudo mv beepboop-* /usr/local/bin/beepboop
beepboop --version
```

Windows PowerShell (`zip`):

```powershell
Invoke-WebRequest -Uri <RELEASE_ASSET_URL> -OutFile beepboop.zip
Expand-Archive -Path .\beepboop.zip -DestinationPath .\beepboop
.\beepboop\beepboop-*.exe --version
```

### Option 2: Install with Go

```bash
go install github.com/jordantrizz/beepboop/cmd/beepboop@latest
```

Make sure your Go bin directory is in `PATH` (commonly `$(go env GOPATH)/bin` or `$(go env GOBIN)`).

### Option 3: Build from source

```bash
git clone https://github.com/jordantrizz/beepboop.git
cd beepboop
VERSION=$(tr -d '[:space:]' < VERSION)
go build -ldflags "-X main.version=${VERSION}" -o beepboop ./cmd/beepboop
```

## Usage

### Verify installation

```bash
beepboop --version
```

### Check a host once

```bash
beepboop --target 1.1.1.1 --mode icmp --once
```

### Check a website once

```bash
beepboop --target https://example.com --mode auto --once
```

### Keep polling until up (default behavior)

```bash
beepboop --target my-host.local --mode auto --interval 5s --timeout 3s
```

### HTTP status-code validation example

```bash
beepboop --target https://example.com --mode auto --status 200,204 --once
```

### Multiple checks — wait until ICMP + TCP ports are all up

```bash
beepboop --target myserver.local --checks icmp,tcp:22,tcp:80
```

### Multiple checks once — verify host is reachable on multiple ports

```bash
beepboop --target 192.168.1.1 --checks icmp,tcp:22,tcp:443 --once
```

### Output examples

Status meanings:
- `target is up` (green)
- `target is down` (yellow)
- `still waiting` (cyan)
- `check failed` (red)

Force plain output:

```bash
beepboop --target https://example.com --once --no-color
```

## CLI Flags
- `--target` target host, IP, or URL (required)
- `--mode` `auto|icmp|http|https|tcp|udp` (default: `auto`; cannot be used with `--checks`)
- `--checks` comma-separated check specs, e.g. `icmp,tcp:22,tcp:80` — all must pass for target to be considered up (uses `--target` as base host; cannot be used with `--mode` or `--port`)
- `--interval` polling interval (default: `5s`)
- `--timeout` per-check timeout (default: `3s`)
- `--retries` extra retry attempts per poll cycle (default: `0`)
- `--once` perform one check and exit
- `--status` expected HTTP status codes, comma-separated (for HTTP/HTTPS checks)
- `--quiet` suppress non-essential output
- `--no-color` force plain output (disable ANSI colors)

### `--checks` spec format

Each spec in the comma-separated `--checks` list takes one of these forms:

| Spec | Description |
|------|-------------|
| `icmp` | ICMP ping to the base host |
| `tcp:PORT` | TCP connection to base host on PORT |
| `udp:PORT` | UDP probe to base host on PORT |
| `http` | HTTP GET to `http://base host` |
| `https` | HTTPS GET to `https://base host` |

## Development

### Run tests

```bash
go test ./...
```

### Local smoke test

```bash
./scripts/dev.sh
./scripts/smoke.sh
```

## CI and Releases
- CI matrix builds run on pushes and pull requests via [ci.yml](.github/workflows/ci.yml)
- Tagged releases (for example `0.2.0`) publish multi-arch artifacts and checksums via [release.yml](.github/workflows/release.yml)

## Notes
Terminal beep behavior depends on terminal settings. If you do not hear a sound, verify your terminal bell/alert preferences.
Color output is automatically disabled for non-interactive terminals and when `NO_COLOR` is set.
