# AGENTS.md

## Purpose
Define execution ownership for building **beepboop**: a cross-platform terminal app/command that checks a host (ICMP and website HTTP/HTTPS) and emits a terminal beep when the target is up.

## Project Defaults
- Language/runtime: **Go** (single static binary, strong cross-compilation, low dependency footprint)
- CLI command name: `beepboop`
- Supported checks:
  - `icmp` for host/IP reachability
  - `http`/`https` for website availability and optional status-code validation
- Default mode behavior:
  - Auto-detect host/IP vs URL target when `--mode=auto`
  - Follow HTTP/HTTPS redirects during website checks
  - Keep polling by default until the target is up (use `--once` for single check)
- Build/release implementation details (target matrix, artifact format, checksum steps) live in `TODO.md`

---

## Agent Roster

### 1) Product Agent
**Mission:** Keep scope tight and usable.

**Owns:**
- CLI UX and flags (`--target`, `--mode`, `--interval`, `--timeout`, `--retries`, `--once`, `--status`, `--quiet`)
- Default runtime contract: run continuously until success unless `--once` is provided
- Error messages and exit-code contract
- MVP scope control (no extra features before Phase 3 gate)

**Definition of done:**
- User can run one command and get clear pass/fail + beep-on-up behavior.

---

### 2) Runtime Agent
**Mission:** Build robust command/runtime behavior.

**Owns:**
- Command parsing and config defaults
- Poll/retry loops and cancellation (`Ctrl+C` handling)
- Beep strategy abstraction:
  - ANSI BEL (`\a`) fallback
  - Optional OS-specific beep adapters only if needed

**Definition of done:**
- Consistent behavior across Linux/macOS/Windows terminals.

---

### 3) Network Agent
**Mission:** Implement network checks correctly and efficiently.

**Owns:**
- ICMP strategy (prefer non-root-safe approach where possible)
- HTTP/HTTPS checks with timeout and status validation
- Auto-detection logic for host/IP vs URL when mode is `auto`
- Redirect handling policy for HTTP/HTTPS checks (follow redirects by default)
- DNS and transient failure handling

**Definition of done:**
- Reproducible up/down detection for both host and website modes.

---

### 4) Build & Release Agent
**Mission:** Own CI/release strategy and reproducibility.

**Owns:**
- CI/release strategy and required quality gates
- Reproducible build policy
- Release publication policy

**Definition of done:**
- CI gates are green and release flow is repeatable for required targets in `TODO.md`.

---

### 5) Local Dev Agent
**Mission:** Keep inner-loop development fast.

**Owns:**
- `scripts/dev.sh` quick local build/test for host OS
- Optional `scripts/dev.ps1` parity for Windows contributors
- One-command local smoke test

**Definition of done:**
- Local contributor can build + smoke test in <30s on a warm machine.

---

### 6) QA Agent
**Mission:** Enforce confidence gates.

**Owns:**
- Unit tests for parsers, checkers, and retry logic
- Integration/smoke tests for HTTP mode
- CI-required checks and coverage floor

**Definition of done:**
- CI passes on PR; flaky tests tracked and minimized.

---

### 7) Docs Agent
**Mission:** Keep setup and usage frictionless.

**Owns:**
- README quick start
- Examples for host and website checks
- Troubleshooting beep limitations by terminal

**Definition of done:**
- New user can install/run from docs without guesswork.

---

## Coordination Rules
- Work in phases from `TODO.md`; do not skip acceptance criteria.
- Keep PRs small and reversible.
- No release until CI matrix and smoke tests are green.
- Prioritize cross-platform correctness over adding features.

## Phase Gates (Summary)
- Gate A: MVP CLI works locally with default run-until-success behavior.
- Gate B: ICMP + HTTP checks stable with tests.
- Gate C: CI workflow requirements in `TODO.md` are green.
- Gate D: Release requirements in `TODO.md` are published.
