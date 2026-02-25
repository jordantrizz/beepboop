# beepboop

`beepboop` is a cross-platform terminal command that checks a host or website and emits a terminal beep when the target is up.

## Features
- Checks host/IP reachability with `icmp`
- Checks websites with `http`/`https`
- Auto-detects host vs URL when `--mode auto` (default)
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
go build -o beepboop ./cmd/beepboop
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
- `--mode` `auto|icmp|http|https` (default: `auto`)
- `--interval` polling interval (default: `5s`)
- `--timeout` per-check timeout (default: `3s`)
- `--retries` extra retry attempts per poll cycle (default: `0`)
- `--once` perform one check and exit
- `--status` expected HTTP status codes, comma-separated (for HTTP/HTTPS mode)
- `--quiet` suppress non-essential output
- `--no-color` force plain output (disable ANSI colors)

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
- Tagged releases (`v*`) publish multi-arch artifacts and checksums via [release.yml](.github/workflows/release.yml)

## Notes
Terminal beep behavior depends on terminal settings. If you do not hear a sound, verify your terminal bell/alert preferences.
Color output is automatically disabled for non-interactive terminals and when `NO_COLOR` is set.
