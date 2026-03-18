# GitHub Release Workflow Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a GitHub Actions workflow that builds and publishes RTLog binaries as draft releases when a version tag is pushed.

**Architecture:** Single workflow file triggered by `v[0-9]*` tags. Reuses existing `make test` and `make release` targets. Creates a draft GitHub Release with auto-generated notes and 4 cross-platform binaries attached.

**Tech Stack:** GitHub Actions, Make, Go 1.24

**Spec:** `docs/superpowers/specs/2026-03-18-github-release-workflow-design.md`

---

### Task 1: Create the release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create the workflow directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Write the workflow file**

Create `.github/workflows/release.yml` with the following content:

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

- [ ] **Step 3: Validate the YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"`
Expected: No output (valid YAML)

If `pyyaml` is not available, use: `go run github.com/google/yamlfmt/cmd/yamlfmt@latest -lint .github/workflows/release.yml` or visually confirm indentation is correct.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add tag-driven release workflow"
```

### Task 2: Verify end-to-end with a test tag

This task is manual and happens after pushing the commit to the remote.

- [ ] **Step 1: Push the workflow to the remote**

```bash
git push origin main
```

- [ ] **Step 2: Create and push a test tag**

```bash
git tag v0.0.1-rc1
git push origin v0.0.1-rc1
```

- [ ] **Step 3: Monitor the workflow run**

```bash
gh run list --limit 1
gh run watch
```

Expected: Workflow completes successfully. A draft release appears at the repository's Releases page with 4 binaries attached.

- [ ] **Step 4: Verify the draft release**

```bash
gh release view v0.0.1-rc1
```

Expected: Status shows `draft`, 4 assets listed.

- [ ] **Step 5: Clean up the test release and tag**

```bash
gh release delete v0.0.1-rc1 --yes
git push origin --delete v0.0.1-rc1
git tag -d v0.0.1-rc1
```
