# PRD — Veil CLI, first OpenCode experiment

Status: experimental; approved pilot target: OpenCode. Final isolation architecture pending tests.

## Goal
Provide a Go wrapper to run OpenCode and later other CLIs with a deliberately minimal, explicit view of the computer's filesystem, environment and network. Device and network identity protections are separate concerns.

## First milestone: offline Linux sandbox
- AC01 — Without bwrap or on unsupported OS, refuse unisolated execution. Test: simulate missing sandbox prerequisite and assert an error.
- AC02 — Default deny network. Test (pending real Linux integration): DNS and outbound TCP fail inside sandbox.
- AC03 — No direct access to host HOME. Test (pending real Linux integration): reading files outside the bound workspace is rejected.
- AC04 — Workspace is the only writable host bind. Test: verify construction and write a disposable file within mounted workspace.
- AC05 — Inherited secrets are cleared. Test (pending integration): parent API token and SSH_AUTH_SOCK are absent inside guest.
- AC06 — Child commands inherit isolation. Test (pending integration): child spawned by shell cannot reach host files or network.
- AC07 — Exits propagate. Test: wrapper mirrors child exit codes. Signal/TUI tests pending.

## Out of scope now
IP rotation, Tor, Raspberry gateway, macOS VM backend, keychain providers, global network interception, tamper-proof protection against privileged processes, anonymous authentication.

## Retrospective placeholder
Fill in after the first real Linux host integration run: successes, difficulties, next step.
