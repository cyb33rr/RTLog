# GitHub Release Workflow Design

## Overview

Automate GitHub Releases for RTLog using a tag-driven GitHub Actions workflow. When a version tag is pushed, CI builds cross-platform binaries and creates a draft release with auto-generated notes.

## Developer Workflow

1. Tag a commit: `git tag v0.1.0`
2. Push the tag: `git push origin v0.1.0`
3. GitHub Actions builds all binaries and creates a draft release
4. Review and edit release notes on GitHub, then publish

## Workflow File

Single file: `.github/workflows/release.yml`

**Trigger:** Tags matching `v*`

**Runner:** `ubuntu-latest`

**Steps:**

1. **Checkout** — `actions/checkout@v4` with `fetch-depth: 0` (required for `git describe --tags`)
2. **Setup Go** — `actions/setup-go@v5` with Go 1.24
3. **Build** — `make release` (existing Makefile target, cross-compiles with `CGO_ENABLED=0`)
4. **Create draft release** — `gh release create` with `--draft --generate-notes`, uploading the 4 binaries

## Build Artifacts

The existing Makefile `release` target produces:

- `rtlog-linux-amd64`
- `rtlog-linux-arm64`
- `rtlog-darwin-amd64`
- `rtlog-darwin-arm64`

These names match what the update system's `FindAssetURL` expects (`rtlog-{os}-{arch}`), so the self-update feature works without changes.

## Version Injection

Uses the existing Makefile mechanism: `git describe --tags` feeds `VERSION`, which is injected via `-X` ldflags into `cmd.Version`.

## Authentication

`GITHUB_TOKEN` is provided automatically by GitHub Actions. No additional secrets configuration needed.

## What Does NOT Change

- Makefile `release` target — reused as-is
- `internal/update/` package — asset naming already compatible
- `cmd/update.go` — no changes needed

## Design Decisions

- **Draft releases** over auto-publish: gives the developer a chance to review and edit auto-generated notes before publishing.
- **Plain GitHub Actions** over GoReleaser: no new dependencies, reuses existing Makefile, right level of complexity for 4 binaries.
- **No checksums**: the update system verifies binaries via ELF/Mach-O magic bytes; checksums can be added later if needed.
