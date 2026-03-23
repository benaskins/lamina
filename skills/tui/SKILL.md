---
name: tui-iterate
description: Use when iterating on Bubble Tea TUI code. Every change gets a direct model test before committing — send messages, assert state and View().
---

# TUI Iterate

Like `/iterate` but for Bubble Tea interfaces. Every UI change gets a model test before you claim it works.

## Cycle

1. **Test first.** Construct the Model, send `tea.Msg` via `Update()`, assert on model state and `View()` output.
2. **Red.** Show the failing test.
3. **Green.** Make the change. Show the passing test.
4. **Verify.** Full test suite. `just install`. Commit.
5. Next change.

## Testing rules

- Always send `tea.WindowSizeMsg{Width: 80, Height: 24}` before checking `View()`.
- Strip ANSI with `charmbracelet/x/ansi` before string assertions. Set `lipgloss.SetColorProfile(termenv.Ascii)` in test init.
- Check `tea.KeyMsg` types against the textarea `KeyMap` — if the textarea binds the key, it will swallow it before your handler sees it.
- Test both the shortcut AND the slash command for every action.
- Returned `tea.Cmd` does NOT execute in direct tests. Assert on model state, not side effects.
- For edit mode: assert entry (`m.editing`), editor content (`m.editor.Value()`), save, and cancel paths.

## Key construction

```go
tea.KeyMsg{Type: tea.KeyCtrlK}          // ctrl+k
tea.KeyMsg{Type: tea.KeyEnter}          // enter
tea.KeyMsg{Type: tea.KeyEscape}         // esc
tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")} // character
```

## Stop and surface if

- A keybinding conflicts with a bubbles component
- View() output doesn't contain expected content after a state change
- A phase transition leaves stale state (entries, viewport content, waiting flag)
