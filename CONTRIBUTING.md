# Contributing

## Before you start

Open an issue with one of the [templates](https://github.com/joaaomanooel/veil/issues/new/choose) before a large change, so the design can be agreed first:
- **Funcionalidade:** a feature.
- **Bug:** a defect.
- **Spike:** a time-boxed investigation.
- **Iniciativa:** an epic with sub-issues.

Issues are written in Portuguese, and code, commits and pull requests in English.

## Build and test

```sh
go vet ./...
go test ./...
CGO_ENABLED=0 go build -o veil ./cmd/veil
```

The sandbox tests need Linux with `/usr/bin/bwrap` and unprivileged user namespaces. On other systems they are skipped, so run them on Linux before opening a pull request that touches `internal/sandbox` or `cmd/veil`.

Gateway changes must keep the end-to-end checks from the [README](README.md#gateway):
- the allowlisted domain answers `200`;
- any other domain answers `403`;
- the stack fails closed when the tunnel is down.

## Rules

- **Fail closed.** If a safeguard is unavailable, refuse to run. Never fall back to running the tool without isolation.
- **Dependencies.** Prefer the Go standard library. Every new dependency, base image or GitHub Action needs a pinned version (a digest or SHA where possible) and a vulnerability check.
- **No secrets or personal data.** Never commit keys, tokens, WARP account files, `.env` files or real IP addresses. Use placeholders such as `gateway.local` and `192.168.0.0/24`.
- **Tests.** Write the test first for new behavior, and add a test that fails without the fix for every bug fix.
- **Documentation site.** `site/` has no build step and makes no external requests. Keep a first visit to any page, with all the CSS and JavaScript it loads, at or under 20 KB with brotli, and each HTML page at or under 6 KB with brotli.
- **Commits.** Use [Conventional Commits](https://www.conventionalcommits.org/), such as `feat:`, `fix:`, `docs:` or `chore:`.
- **Pull requests.** Keep them focused, and explain the reason and how you verified the change.

## License

By contributing, you agree that your contributions are licensed under the [GPL-3.0](LICENSE).
