---
name: lamina-workspace
description: Use when you need to understand the lamina workspace layout, check repo health, or investigate cross-repo dependency issues. Extends /ground with lamina-specific tools.
---

# Lamina Workspace

Extends `/ground` with lamina workspace intelligence. Use when navigating between repos, checking health, or diagnosing dependency problems.

## Orientation

```bash
lamina repo                # All repos — branch, clean/dirty, last commit
lamina deps                # Dependency graph between modules
lamina doctor              # Check workspace health
```

## Per-repo and workspace-wide

```bash
lamina repo <name>              # Full git status for one repo
lamina repo <name> rebase       # Git pull --rebase one repo
lamina repo rebase --all        # Git pull --rebase all repos
lamina repo push --all          # Git push all repos (--all required as safety rail)
```

## Cross-repo workflows

```bash
# Find dirty repos
lamina repo list --json | jq '.[] | select(.dirty) | .name'

# Understand a module's dependencies
lamina deps --json | jq '.[] | select(.module | contains("axon-chat"))'

# Run tests across libraries
lamina test                     # All axon-* libraries
lamina test axon axon-auth      # Specific repos
```

## Dependency debugging

When a service won't build after a library change:

1. `lamina deps --json` to see the full chain
2. Check the service's `go.mod` for stale `replace` directives
3. `go mod tidy` in the service directory
4. `lamina test <library>` to verify the library itself passes

$ARGUMENTS
