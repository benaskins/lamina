---
name: verify-lamina
description: Use before claiming any fix, release, or implementation is complete in the lamina workspace. Extends /verify with lamina-specific evidence commands.
---

# Verify Lamina

Follow `/verify` — here's how to show evidence in the lamina workspace.

## Evidence commands

```bash
lamina doctor              # Workspace health — stale deps, version mismatches
lamina test                # Full test suite across all axon-* modules
lamina test <repo>         # Test a specific module
lamina repo                # All repos — branch, clean/dirty, last commit
```

## After a library change

1. `lamina test <library>` — the changed module passes
2. `lamina test` — downstream consumers still pass
3. `lamina doctor` — no version mismatches introduced

## After a release

1. `lamina doctor` — workspace is consistent
2. `lamina repo` — all repos clean, no unpushed tags

## After a deploy

1. `curl -s` the health endpoint — show the response
2. `lamina doctor` — workspace matches what's running

$ARGUMENTS
