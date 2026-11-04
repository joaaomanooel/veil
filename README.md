# Veil — OpenCode isolation experiment (MVP 0)

Initial experimental security harness written in Go. **Not a production anonymity tool.** It runs a command in a **Linux-only** bubblewrap sandbox, either fully offline or with a single egress path through an allowlisted proxy (the Raspberry Pi Tor gateway in `deploy/pi/gateway`). The macOS backend, credentials management, and IP rotation **are not implemented**.

## Requirements
- Go 1.22+
- Linux with user namespaces enabled and `bubblewrap` (`bwrap`) installed
- An existing, dedicated non-sensitive project directory

## Build / test

```sh
go test ./...
go build -o veil ./cmd/veil
./veil doctor
mkdir -p /tmp/veil-test-workspace
./veil run --profile offline --workspace /tmp/veil-test-workspace -- /bin/sh -c 'printf "HOME=%s\\n" "$HOME"; hostname; ls /home; ls /workspace'
```

Run `./veil run --profile offline --workspace /path/to/a/test-project -- opencode --version` only **after** verifying which OpenCode version and binary packaging are installed. The offline sandbox deliberately prevents LLM calls; it might need runtime-specific mounts for OpenCode to launch. A Node/npm shim or dependencies outside `/usr` will not automatically be accessible. Never run with `sudo`.

### Gateway profile

```sh
./veil run --profile gateway --gateway 192.168.0.11:8118 --workspace /path/to/a/test-project -- opencode run "..."
```

The sandbox still has no network interfaces. `veil` checks that the gateway answers, binds a host unix socket into the sandbox, and an internal `veil bridge` exposes it as `HTTPS_PROXY=http://127.0.0.1:8118` (`NO_PROXY=127.0.0.1,localhost` keeps OpenCode V2's local server off the proxy). If the gateway is unreachable, `veil` refuses to start the command. Build `veil` with `CGO_ENABLED=0` because the same binary runs inside the sandbox.

## Security invariants (MVP 0)
- Refuse to run unless Linux and `bwrap` are available. Never fall back to `exec` on the host.
- Use an isolated network namespace with no network interfaces except loopback. In the gateway profile the only way out is the bound proxy socket.
- Mount only a chosen workspace read-write, core OS binaries/libraries read-only, selected executable read-only, isolated `/home`, `/tmp`, `/proc`, and `/dev`.
- Clear inherited environment, including API tokens, `SSH_AUTH_SOCK`, and proxy variables.
- Reject mounting `/` or the user's full home directory as workspace.

## Known limitations
- The workspace contents, including any `.env` or credentials you place there, **are visible to the guest**. Test only with disposable projects.
- `bubblewrap` and user namespaces depend on host kernel settings; a system can refuse sandbox startup. This must stay fail-closed.
- Only OpenCode V2 (`v2.1.4`) was exercised, through the gateway profile. Its internal permissions are **not** a substitute for OS sandboxing.
- No persistent sandbox state, credentials, project access policy, or TUI-specific hardening yet. Signals are forwarded but not tested with the TUI.
- A tool that ignores `HTTPS_PROXY` cannot reach the network at all in the gateway profile; it fails instead of leaking.
- Isolation does not guarantee an unidentifiable computer: tool calls, file contents, account identifiers and behavior can still correlate sessions.
- No guarantee of malware resistance in the presence of kernel exploits, privileged hosts, or trust in mounted project files.
- A host running OpenCode outside `veil` remains unrestricted by this tool.

## Next iteration
1. Detect OpenCode installation variant **without executing** unknown binaries; explicitly test V1 vs V2 config.
2. Create disposable sandbox-specific XDG auth/config storage and interactive TUI support.
3. Add integration tests for filesystem, network (TCP/DNS), subprocess containment, and exit/signal behavior on real Linux hosts with bwrap.
4. Add traffic/DNS/IPv6 audits of the gateway profile.
5. Add a Linux VM backend for macOS (do not run Mach-O executables in Linux).

Sources: https://opencode.ai/docs/en/config/ , https://dev.opencode.ai/v2/docs/permissions/ , https://github.com/containers/bubblewrap
