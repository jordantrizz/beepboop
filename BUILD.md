# BUILD.md

## Purpose
Track the initial build/release implementation plan and completion status for `beepboop`.

## Scope (Initial Build)
- Bootstrap project structure
- Implement MVP CLI runtime behavior
- Implement network check engine
- Add local development build/test scripts
- Add CI matrix builds
- Add release automation

---

## Phase 0 — Project Bootstrap
**Status:** Complete

- [x] Initialize Go module and CLI entrypoint (`cmd/beepboop`)
- [x] Add basic folder layout (`internal/check`, `internal/beep`, `scripts`, `.github/workflows`)
- [x] Add formatting/linting baseline (`gofmt`, optional `golangci-lint`)
- [x] Add README stub + usage placeholder

**Exit criteria:**
- [x] Repo builds locally with `go build ./...`
- [x] `beepboop --help` runs

---

## Phase 1 — MVP Command Behavior
**Status:** Complete

- [x] Implement CLI flags:
  - [x] `--target` (host, IP, or URL)
  - [x] `--mode` (`auto|icmp|http|https`)
  - [x] `--interval` (default polling interval)
  - [x] `--timeout` (per-attempt timeout)
  - [x] `--retries` (before reporting down)
  - [x] `--once` (single check then exit; overrides default continuous polling)
- [x] Add simple terminal beep (`\a`) on success
- [x] Set default loop behavior to continue until target is up
- [x] Add meaningful exit codes and concise output

**Exit criteria:**
- [x] Can detect up/down and beep in local terminal, defaulting to run-until-success

---

## Phase 2 — Network Check Engine
**Status:** Complete

- [x] Implement ICMP host checker
- [x] Implement HTTP/HTTPS checker with status validation (`--status 200,204,...`)
- [x] Add target normalization (auto-detect URL vs host)
- [x] Follow HTTP/HTTPS redirects by default (bounded redirect limit)
- [x] Add retry/backoff behavior for transient failures
- [x] Add unit tests for parsing, normalization, redirect handling, and checker logic

**Exit criteria:**
- [x] Host and website checks are stable under timeout/retry scenarios, including redirect paths

---

## Phase 3 — Local Fast Build/Test Scripts
**Status:** Mostly complete

- [x] Add `scripts/dev.sh`:
  - [x] Detect current OS/arch
  - [x] Build binary to `./dist/local/`
  - [x] Run fast test set (or smoke tests)
  - [x] Print exact binary path and usage hint
- [ ] (Optional) Add `scripts/dev.ps1` equivalent for Windows contributors
- [x] Add `scripts/smoke.sh` against known hosts/URLs

**Exit criteria:**
- [x] Contributor runs one script and gets local build + quick verification

---

## Phase 4 — GitHub CI Matrix Builds
**Status:** Complete

- [x] Add CI workflow on push/PR:
  - [x] Lint + unit tests
  - [x] Build matrix:
    - [x] `linux/amd64`, `linux/arm64`
    - [x] `darwin/amd64`, `darwin/arm64`
    - [x] `windows/amd64`, `windows/arm64`
- [x] Upload build artifacts for each matrix target
- [x] Cache Go modules/build cache for speed

**Exit criteria:**
- [x] Every PR validates and produces matrix artifacts

---

## Phase 5 — Release Automation
**Status:** Complete

- [x] Add release workflow (`v*` tags)
- [x] Build all target binaries with version metadata (`-ldflags`)
- [x] Package artifacts (`zip`/`tar.gz`) + SHA256 checksums
- [x] Publish GitHub Release with attached artifacts

**Exit criteria:**
- [x] Tagging `vX.Y.Z` publishes complete release assets automatically

---

## Initial Build Milestones
- [x] M1: Phase 0-1 complete (single-target MVP)
- [x] M2: Phase 2-3 complete (robust checks + local workflow)
- [x] M3: Phase 4-5 complete (CI matrix + automated releases)
