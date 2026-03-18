# GitHub Release Workflow Design

## Overview

Automate GitHub Releases for RTLog using a tag-driven GitHub Actions workflow. When a version tag is pushed, CI builds cross-platform binaries and creates a draft release with auto-generated notes.

## Developer Workflow

1. Tag a commit: `git tag v0.1.0`
2. Push the tag: `git push origin v0.1.0`
3. GitHub Actions runs tests, builds all binaries, and creates a draft release
4. Review and edit release notes on GitHub, then publish
5. The self-update system sees the new version only after publishing (draft releases are not returned by the `/releases/latest` API endpoint — this is intentional to prevent incomplete releases from reaching users)

## Workflow File

Single file: `.github/workflows/release.yml`

**Trigger:** Tags matching `v[0-9]*` (prevents accidental triggers from non-version tags)

**Runner:** `ubuntu-latest`

**Permissions:** `contents: write` (required for creating releases and uploading assets)

**Steps:**

1. **Checkout** — `actions/checkout@v4` with `fetch-depth: 0` (required for `git describe --tags`)
2. **Setup Go** — `actions/setup-go@v5` with Go 1.24
3. **Test** — `make test`
4. **Build** — `make release`
5. **Create draft release** — `gh release create` with `--draft --generate-notes --verify-tag`, uploading the 4 binaries

## Target YAML

```yaml
name: Release

on:
  push:
    tags:
      - "v[0-9]*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Test
        run: make test

      - name: Build
        run: make release

      - name: Create draft release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: >
          gh release create "${{ github.ref_name }}"
          rtlog-linux-amd64
          rtlog-linux-arm64
          rtlog-darwin-amd64
          rtlog-darwin-arm64
          --draft
          --generate-notes
          --verify-tag
```

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

`GITHUB_TOKEN` is provided automatically by GitHub Actions. The workflow declares `permissions: contents: write` (required since GitHub defaults to read-only). The token is passed to `gh` via the `GH_TOKEN` environment variable.

## What Does NOT Change

- Makefile `release` target — reused as-is
- `internal/update/` package — asset naming already compatible
- `cmd/update.go` — no changes needed

## Design Decisions

- **Draft releases** over auto-publish: gives the developer a chance to review and edit auto-generated notes before publishing. Draft releases are invisible to the `/releases/latest` API endpoint, so the self-update system only sees the release after the developer publishes it.
- **Plain GitHub Actions** over GoReleaser: no new dependencies, reuses existing Makefile, right level of complexity for 4 binaries.
- **No checksums**: the update system verifies binaries via ELF/Mach-O magic bytes; checksums can be added later if needed.
- **Test before build**: `make test` runs before `make release` to prevent shipping broken code. This is the only CI gate since no other workflows exist.
- **Tag pattern `v[0-9]*`** over `v*`: prevents accidental triggers from non-semver tags.
