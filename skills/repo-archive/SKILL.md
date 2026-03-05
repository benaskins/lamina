---
name: repo-archive
description: Workflow for archiving repos and projects — move to ~/dev/archive/ instead of deleting, verify no active consumers.
---

# Repo Archive

Step-by-step workflow for safely archiving repos and projects that are no longer actively needed.

## Principle

**Never delete, always archive.** Move to `~/dev/archive/` so things can be recovered.

## Pre-flight

Before archiving:

1. **Check for consumers** — other repos that depend on this one
```bash
lamina deps --json | jq '.[] | select(.deps[] | contains("REPO_NAME"))'
```

2. **Check for uncommitted work**
```bash
lamina repo REPO_NAME
```

3. **Ensure pushed to remote** — archive is local only
```bash
cd ~/dev/REPO_NAME && git log --oneline origin/main..HEAD
```

## Step 1: Archive

```bash
mkdir -p ~/dev/archive
mv ~/dev/REPO_NAME ~/dev/archive/REPO_NAME
```

## Step 2: Verify nothing broke

```bash
lamina doctor        # Check workspace health
lamina deps          # Verify dependency graph still resolves
```

If services had `replace` directives pointing to the archived repo, you have two options:

**Option A: Switch to published GitHub packages** (preferred)
```bash
go mod edit -dropreplace github.com/benaskins/PACKAGE
go mod edit -require github.com/benaskins/PACKAGE@VERSION
GOWORK=off go mod tidy
```

**Option B: Update relative paths** to point into archive (temporary)
```bash
go mod edit -replace github.com/benaskins/PACKAGE=../../archive/REPO_NAME
```

## Step 3: Tag if needed

If switching to published packages and the archived module lacks version tags:

```bash
cd ~/dev/archive/REPO_NAME
git tag v0.1.0
git push origin v0.1.0
```

Or from the workspace: `lamina release REPO_NAME v0.1.0`

## Recovery

To restore an archived repo:

```bash
mv ~/dev/archive/REPO_NAME ~/dev/REPO_NAME
```

## What's in the archive

```bash
ls ~/dev/archive/
```

The archive is not version-controlled — it's a local holding area. Each archived item retains its own `.git` history.
