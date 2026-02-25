# TODO.md
Track remaining work only. Initial build implementation history lives in `BUILD.md`.

## Small Changes
- [x] Colorized terminal output
	- [x] Define a minimal color scheme for `up`, `down`, `waiting`, and `error` states
	- [x] Add ANSI color output with automatic fallback to plain text when color is disabled/not supported
	- [x] Add `--no-color` flag to force plain output
	- [x] Update README examples/screenshots to reflect colorized output behavior

- [ ] Compact runtime progress display
	- [ ] Track and display run `start time`
	- [ ] Track and display `current time` and `elapsed duration`
	- [ ] Track and display check attempts and retry counts in a compact single-line status view
	- [ ] Ensure progress output works cleanly in both TTY and non-TTY environments

- [ ] Smarter auto-detection for target input
	- [ ] Keep explicit `http://` and `https://` targets mapped to HTTP/HTTPS mode
	- [ ] Detect plain IP/hostname targets and map to ICMP mode
	- [ ] Detect URL-like bare targets (e.g. domain/path, domain:port) and default to HTTP/HTTPS mode
	- [ ] Add tests covering ambiguous and edge-case targets

- [ ] Verbose and machine-readable output
	- [ ] Add `--verbose` mode with per-attempt diagnostic details (mode, resolved target, status/error, retry info)
	- [ ] Add `--json` output mode for structured logs/results
	- [ ] Define and document JSON schema fields for single-check and run-until-success flows
	- [ ] Add tests validating JSON output format and key fields

## Unfinished Build Items
- [ ] Add `scripts/dev.ps1` equivalent for Windows contributors
- [ ] Make smoke test network-independent in CI (local test target instead of external site)
- [ ] Add `golangci-lint` (or `staticcheck`) as explicit CI gate
- [ ] Add `go test -race ./...` on Linux `amd64` in CI
- [ ] Add release provenance/SBOM generation

## Unfinished Product/UX Items
- [ ] Add verbose mode and machine-readable output (`--json` optional)
- [ ] Colorify output so status is easier to scan
- [ ] Show compact runtime progress (start time, elapsed time, retry progress)
- [ ] Improve auto-detection so bare URL-like targets (without scheme) default to HTTP/HTTPS instead of ICMP when appropriate

## Backlog (Post-MVP)
- [ ] Config file support (`beepboop.yaml`)
- [ ] Notification adapters beyond terminal beep (desktop/webhook)
- [ ] Homebrew/Scoop/package manager distribution
- [ ] Optional install script for latest release
