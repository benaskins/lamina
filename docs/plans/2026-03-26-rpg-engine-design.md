# RPG Engine — Text-Based Dungeon Master via Axon

**Date**: 2026-03-26
**Status**: Draft
**Rule System**: [Cairn](https://cairnrpg.com/) (CC-BY-SA 4.0)

## Premise

A text-based RPG where an LLM acts as dungeon master (Warden in Cairn terms). The player's interface is a conversation. The Warden narrates, adjudicates rules, voices NPCs, and drives the world forward — all constrained by game mechanics executed through tool calls.

This is **axon-chat with a game layer**. The conversation loop IS the game loop.

```
Player types "I try to pick the lock on the chest"
  → axon-loop sends to LLM (with Warden system prompt + tools)
  → LLM calls save(character="Aldric", ability="dex")
  → Tool rolls d20 vs DEX 14, returns "Roll: 11 vs DEX 14 — Success."
  → LLM narrates: "The tumblers click into place. Inside, you find..."
  → LLM calls add_to_inventory(character="Aldric", item="Ruby Amulet")
  → Streamed token-by-token via SSE to the player
```

## Why Cairn

Cairn's rules fit on a page. The entire mechanical layer is ~5 Go functions. This means:
- The LLM spends context on narrative, not bookkeeping
- Combat resolves in 1-3 rounds (deadly), not 10 (grindy)
- The full rules can be embedded in the system prompt
- Tool surface is minimal — fewer chances for the LLM to misuse mechanics

## Architecture

```
┌──────────────────────────────────────────────┐
│  axon — HTTP server, SSE streaming, auth     │
│  POST /api/chat          (game turn)         │
│  GET  /api/game/state    (current state)     │
│  GET  /api/game/character (character sheet)   │
└──────────────────┬───────────────────────────┘
                   │
┌──────────────────▼───────────────────────────┐
│  axon-loop — Conversation loop               │
│  System prompt: Cairn rules + world lore     │
│  Tools: game mechanic functions              │
│  Context: SlidingWindow or TokenBudget       │
│  Callbacks: OnToken → SSE, OnToolUse → log   │
└──────────────────┬───────────────────────────┘
                   │ tool calls
        ┌──────────┼──────────┐
        ▼          ▼          ▼
   Go funcs    axon-fact   axon-memo
   (rules)     (state)     (NPC memory)
```

## Cairn Rules as Go Functions

### The Complete Mechanical Layer

Cairn has 3 stats (STR, DEX, WIL), d20-under saves, auto-hit combat, and inventory slots. Here's the full rules engine:

```go
// Save: roll d20, succeed if equal to or under ability score.
// 1 always succeeds, 20 always fails.
func save(ability int) (roll int, passed bool) {
    roll = rand.Intn(20) + 1
    if roll == 1 { return roll, true }
    if roll == 20 { return roll, false }
    return roll, roll <= ability
}

// Damage: roll weapon die, subtract target's armor (max 3), apply to HP.
// If HP hits 0 exactly → Scars table.
// If HP goes below 0 → overflow hits STR, target must make STR save or die.
func damage(weaponDie int, armor int) int {
    raw := rand.Intn(weaponDie) + 1
    return max(0, raw - min(armor, 3))
}

// Impaired: always roll d4 regardless of weapon (fighting from weakness)
// Enhanced: always roll d12 regardless of weapon (fighting from advantage)
// Dual weapons / multiple attackers: roll all dice, keep highest

// Morale: enemies WIL save on first casualty, again at half strength.
// Lone foes save at 0 HP.

// Reactions (2d6): 2=Hostile, 3-5=Wary, 6-8=Curious, 9-11=Kind, 12=Helpful
```

That's it. The entire combat and resolution system.

### Character Model

```go
type Character struct {
    Name       string
    Background string   // Alchemist, Blacksmith, Burglar, etc.
    STR, DEX, WIL int   // 3d6 each at creation
    MaxSTR, MaxDEX, MaxWIL int
    HP, MaxHP  int       // 1d6 at creation
    Armor      int       // 0-3, from equipment
    Gold       int       // 3d6 starting
    Inventory  [10]Slot  // 4 body + 6 backpack
    Deprived   bool      // Cannot recover HP if true
    Traits     Traits    // Physique, skin, hair, face, speech, etc.
}

type Slot struct {
    Item   string
    Bulky  bool   // Takes 2 slots
    Fatigue bool  // From spellcasting or deprivation
}

// Full inventory (10 slots filled) → HP reduced to 0
// Bulky items take 2 slots, typically two-handed
// Spellcasting adds 1 Fatigue to inventory (occupies 1 slot)
```

### Character Creation (all random)

```
Age:        2d20 + 10
STR:        3d6 (may swap any two of STR/DEX/WIL)
DEX:        3d6
WIL:        3d6
HP:         1d6
Gold:       3d6
Background: d20 table (Alchemist, Blacksmith, Burglar, Butcher, ...)
Traits:     d10 each (physique, skin, hair, face, speech, clothing,
            virtue, vice, reputation, misfortune)
Armor:      d20 (1-3: none, 4-14: brigandine/1, 15-19: chain/2, 20: plate/3)
Weapon:     d20 (1-5: d6, 6-14: d8, 15-19: d6 ranged or d8 ranged, 20: d10 bulky)
Gear:       d20 expeditionary + d20 tools + d20 trinkets
Bonus:      d20 (1-5: tool/trinket, 6-13: gear, 14-17: armor/weapon, 18-20: spellbook)
```

### Combat Flow

```
1. First round: each PC makes DEX save to act before enemies
2. On turn: move 40ft + one action (attack, cast, move again, etc.)
3. Attack: roll weapon die, subtract armor, deal remainder to HP
   - Multiple attackers on same target: roll all, keep highest
   - Dual wielding: roll both, keep highest
   - Impaired (disadvantage): roll d4 regardless of weapon
   - Enhanced (advantage): roll d12 regardless of weapon
   - Blast: affects all targets in area, roll separately
4. HP to exactly 0 → Scars table (d12, often beneficial long-term)
5. HP below 0 → remainder hits STR, STR save or critical damage
   - Critical: can only crawl, die within the hour without aid
   - STR 0 = dead, DEX 0 = paralyzed, WIL 0 = delirious
6. Morale: enemies WIL save on first casualty + at half strength
7. Retreat: requires DEX save + safe destination
```

### Magic

```
- Spellbooks: 1 slot each, hold 1 spell, recovered from the world (not created)
- Casting: hold in both hands, read aloud, add 1 Fatigue to inventory
- Enhanced casting: safe + time → stronger effect, no extra cost
- Dangerous casting: deprived or in danger → WIL save to avoid ill effects
- Scrolls: no slot, no fatigue, single use
- Relics: magical items with charges + recharge conditions
```

## Tool Definitions

The Warden's toolbox. Dramatically simpler than D&D:

### Core Mechanics

| Tool | Purpose | Cairn Rule |
|------|---------|------------|
| `save` | d20-under ability check | Roll d20 ≤ ability. 1=auto-pass, 20=auto-fail |
| `roll_dice` | Roll NdS+M for any purpose | Damage, tables, gold, HP, etc. |
| `attack` | Deal weapon damage to target | Roll weapon die - armor → HP (overflow → STR) |
| `cast_spell` | Cast from spellbook | Add Fatigue slot, resolve effect |
| `reaction_roll` | NPC disposition | 2d6: hostile/wary/curious/kind/helpful |
| `morale_check` | Do enemies flee? | WIL save for enemy group |

### State Management

| Tool | Purpose | Returns |
|------|---------|---------|
| `get_character` | Read character sheet | Stats, HP, inventory, conditions |
| `apply_damage` | Deal damage (handles HP→STR overflow + crit) | New HP/STR, scar or crit result |
| `heal` | Rest and recover HP | New HP (or blocked if deprived) |
| `add_to_inventory` | Pick up item | Updated slots, warn if full (full = 0 HP) |
| `drop_item` | Remove item from inventory | Updated slots |
| `add_fatigue` | From deprivation or spellcasting | Updated slots |
| `create_character` | Random character generation | Full character sheet |

### World & NPCs

| Tool | Purpose | Returns |
|------|---------|---------|
| `recall_npc_memory` | What does NPC know/feel about player? | Memories, trust metrics |
| `get_location` | Current location details | Description, exits, NPCs, items |
| `move_to` | Change player location | New location description |
| `check_time` | In-game time and conditions | Time of day, weather, season |

### Example: The `attack` Tool

```go
func attackTool(chars *CharacterProjector) tool.ToolDef {
    return tool.ToolDef{
        Name:        "attack",
        Description: "Roll weapon damage against a target. Attacks auto-hit in Cairn. Roll weapon die, subtract armor, apply to HP. If HP goes below 0, overflow damages STR and target must STR save.",
        Parameters: tool.ParameterSchema{
            Type:     "object",
            Required: []string{"weapon_die", "target"},
            Properties: map[string]tool.PropertySchema{
                "weapon_die": {Type: "integer", Description: "Weapon damage die: 4, 6, 8, 10, or 12", Enum: []any{4, 6, 8, 10, 12}},
                "target":     {Type: "string", Description: "Target character or creature ID"},
                "modifier":   {Type: "string", Description: "impaired (forced d4), enhanced (forced d12), or normal", Enum: []any{"normal", "impaired", "enhanced"}, Default: "normal"},
            },
        },
        Execute: func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
            targetID, _ := args["target"].(string)
            die := int(args["weapon_die"].(float64))
            mod, _ := args["modifier"].(string)

            switch mod {
            case "impaired": die = 4
            case "enhanced": die = 12
            }

            target := chars.Get(targetID)
            raw := rand.Intn(die) + 1
            after_armor := max(0, raw - min(target.Armor, 3))

            result := applyDamage(target, after_armor) // handles HP→STR overflow, crit saves, scars
            return tool.ToolResult{Content: result.Narration()}
        },
    }
}
```

## Event Streams (axon-fact)

Every game action is an immutable event:

```
character:{id}   — CharacterCreated, DamageTaken, Healed, ItemAdded,
                   ItemDropped, FatigueAdded, FatigueCleared,
                   ScarReceived, AbilityChanged, Deprived, Died

encounter:{id}   — EncounterStarted, InitiativeRolled, AttackResolved,
                   SpellCast, MoraleChecked, CreatureDefeated,
                   EncounterEnded

world:{region}   — RegionEntered, EventTriggered, NPCSpawned

campaign:{id}    — CampaignCreated, SessionStarted, SessionEnded
```

**Projectors** build read models:
- **CharacterSheet** — current HP, STR/DEX/WIL, inventory slots, conditions
- **EncounterState** — who's in the fight, creature HP, initiative
- **WorldState** — current region, known locations, world flags

## axon-memo — NPC Memory

NPCs remember the player across sessions. The `recall_npc_memory` tool does vector search when the player talks to an NPC. Relationship metrics (ABI trust model) drive NPC disposition — stacks with Cairn's 2d6 reaction roll.

## axon-nats — Shared World (Multiplayer)

`EventBus[WorldEvent]` fans out world events across server instances. Player A slays a creature, Player B hears about it from an NPC.

## axon-task — Background Processing

- **NarrativeWorker** — session summaries
- **ConsolidationWorker** — nightly NPC memory consolidation
- **WorldTickWorker** — time-based world events

## System Prompt Structure

```
1. Warden Personality & Style
   "You are the Warden for a dark fantasy campaign using Cairn rules.
    Your tone is vivid but concise. Death is real and never random."

2. Cairn Rules Summary (fits in ~500 tokens)
   "Saves: d20 ≤ ability. 1=pass, 20=fail.
    Combat: auto-hit, roll weapon die - armor → HP.
    HP below 0 → STR damage, STR save or critical.
    Armor max 3. Full inventory (10 slots) → 0 HP.
    Magic: spellbook in both hands, adds Fatigue.
    Morale: WIL save on first casualty + half strength."

3. Current State (from projectors)
   "Aldric: STR 12, DEX 14, WIL 9, HP 4/6, Armor 1
    Inventory: dagger(d6), brigandine(bulky), torch, rations,
    spellbook(Fog Cloud), [empty], [empty], [empty], [empty], [empty]
    Location: Rotwood Chapel. Time: Dusk, fog rolling in."

4. NPC Memory Context (from axon-memo)
   "Hermit Voss: Trusts you (0.7). Remembers you brought medicine.
    Knows the location of the iron door beneath the chapel."

5. Tool Usage Guidelines
   "Use tools for ALL mechanical resolution. Never invent dice rolls.
    Call save() for risky actions. Call attack() for combat damage.
    Call get_character() before describing stats or inventory.
    Call recall_npc_memory() before voicing any recurring NPC."
```

## Implementation Plan

Each step is one commit-sized change.

### Phase 1: Foundation

1. **New module `axon-rpg`** — `go mod init`, Cairn types (Character, Creature, Item, Spell)
2. **Dice engine** — `roll_dice` tool + NdS+M parser + tests
3. **Save mechanic** — `save` tool (d20-under) + tests
4. **Character creation** — random generation from Cairn tables + `create_character` tool

### Phase 2: Game Loop

5. **Warden agent** — system prompt builder with embedded Cairn rules, tool wiring, `loop.Run`
6. **Combat tools** — `attack` (with impaired/enhanced/blast), `apply_damage` (HP→STR overflow, scars, crit)
7. **Inventory tools** — `add_to_inventory`, `drop_item`, slot tracking, bulky items, full=0HP
8. **Magic tools** — `cast_spell` (fatigue), spellbook inventory management

### Phase 3: World

9. **Character model** — event-sourced with projector (axon-fact)
10. **Location model** — event-sourced locations, `get_location`, `move_to`
11. **Encounter model** — initiative, morale checks, creature tracking
12. **NPC memory** — `recall_npc_memory` wired to axon-memo

### Phase 4: Service

13. **HTTP handlers** — game session endpoints, SSE streaming (reuse axon-chat patterns)
14. **Campaign persistence** — session start/end, campaign-level events
15. **Multiplayer** — axon-nats for shared world events
16. **Eval plans** — axon-eval scenarios for testing Warden behavior

### Phase 5: Advanced (Optional)

17. **axon-mind for quest graphs** — Prolog for complex prerequisite chains
18. **axon-mind for faction logic** — transitive alliance/enemy inference
19. **Cairn 2e enhancements** — modular rules, advanced scar mechanics

## What Doesn't Need Building

The axon ecosystem already provides:

- HTTP server with graceful shutdown, health checks, metrics → **axon**
- SSE streaming with fan-out → **axon/sse**
- Conversation loop with tool dispatch and streaming → **axon-loop**
- Tool definition framework → **axon-tool**
- Event store with projectors (memory + postgres) → **axon-fact**
- NPC memory with vector search and relationship tracking → **axon-memo**
- Prolog inference engine (phase 5, quest/faction graphs) → **axon-mind**
- Cross-instance event distribution → **axon-nats**
- Background task queue → **axon-task**
- LLM provider adapters (Ollama, Claude, GPT) → **axon-talk**
- Evaluation framework → **axon-eval**
- Auth middleware → **axon**

The RPG-specific code is: Cairn types, dice parser, save/damage/combat functions, inventory slot management, character creation tables, system prompt builder, and HTTP handlers. Everything else is composition.

## References

- [Cairn SRD (1st Edition)](https://cairnrpg.com/first-edition/cairn-srd/) — CC-BY-SA 4.0
- [Cairn 2e](https://cairnrpg.com/second-edition/) — CC-BY-SA 4.0
- [Cairn GitHub](https://github.com/yochaigal/cairn)
