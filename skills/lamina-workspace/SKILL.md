---
name: lamina-workspace
description: Use when you need to understand the lamina workspace layout, check repo health, or investigate cross-repo dependency issues.
---

# Lamina Workspace

Workspace intelligence for the lamina compute cluster. Use when navigating between repos, checking health, or diagnosing dependency problems.

## Orientation

Get a quick picture of the workspace:

```bash
lamina repo                # All repos — branch, clean/dirty, last commit
lamina repo list --json    # Same, as JSON
lamina deps                # Dependency graph between modules
```

## Per-repo operations

```bash
lamina repo axon           # Full git status for axon
lamina repo axon fetch     # Git fetch for axon
lamina repo axon push      # Git push axon
lamina repo axon rebase    # Git pull --rebase axon
```

## Workspace-wide operations

```bash
lamina repo status         # Full git status for every repo
lamina repo fetch          # Git fetch all repos
lamina repo push --all     # Git push all repos (--all required as safety rail)
lamina repo rebase --all   # Git pull --rebase all repos (--all required)
```

## Cross-repo workflows

### Check what's changed across the workspace

```bash
lamina repo list --json | jq '.[] | select(.dirty) | .name'
```

Dirty repos may have uncommitted work that affects other modules.

### Understand a module's dependencies

```bash
lamina deps --json | jq '.[] | select(.module | contains("axon-chat"))'
```

### Run tests across libraries

```bash
lamina test                     # All axon-* libraries
lamina test axon axon-auth      # Specific repos
```

## Common issues

| Symptom | Likely cause |
|---|---|
| Build failure in a service | Dependency updated but not replaced — check `lamina deps` for the chain |
| Tests pass locally but not in service | Service go.mod has stale `replace` directives — compare versions |
| New module not showing in `lamina repo` | Directory exists but `git init` hasn't been run |
| `lamina deps` missing a connection | Module not in `go.mod` `require` block yet |

## Dependency debugging

When a service won't build after a library change:

1. `lamina deps --json` to see the full chain
2. Check the service's `go.mod` for `replace` directives pointing at the right local paths
3. Run `go mod tidy` in the service directory
4. `lamina test <library>` to verify the library itself passes
