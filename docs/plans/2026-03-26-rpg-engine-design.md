# RPG Engine — Text-Based Dungeon Master via Axon

**Date**: 2026-03-26
**Status**: Draft

## Premise

A text-based RPG where an LLM acts as dungeon master. The player's interface is a conversation. The DM narrates, adjudicates rules, voices NPCs, and drives the world forward — all constrained by game mechanics executed through tool calls.

This is **axon-chat with a game layer**. The conversation loop IS the game loop.

```
Player types "I try to pick the lock on the chest"
  → axon-loop sends to LLM (with DM system prompt + tools)
  → LLM calls check_skill(character="Aldric", skill="lockpicking", dc=15)
  → Tool rolls d20 + modifier, returns "Roll: 18 (d20: 14 + DEX: 4). Success."
  → LLM narrates: "The tumblers click into place. Inside, you find..."
  → LLM calls add_to_inventory(character="Aldric", item="Ruby Amulet")
  → Streamed token-by-token via SSE to the player
```

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
│  System prompt: world lore + DM instructions │
│  Tools: game mechanic functions              │
│  Context: SlidingWindow or TokenBudget       │
│  Callbacks: OnToken → SSE, OnToolUse → log   │
└──────────────────┬───────────────────────────┘
                   │ tool calls
        ┌──────────┼──────────┐
        ▼          ▼          ▼
   axon-mind   axon-fact   axon-memo
   (rules)     (state)     (NPC memory)
```

## Module Roles

### axon-loop + axon-tool — The DM's Voice

The LLM is the dungeon master. `loop.Run()` orchestrates each turn:

- **System prompt** contains world lore, campaign context, DM personality, and rules summary
- **Tools** are the DM's interface to game mechanics — the LLM never modifies state directly, only through tools
- **Context strategy** manages long campaigns: `SlidingWindow(20)` keeps recent turns, or `TokenBudgetWithMinWindow` for flexible memory
- **Streaming** delivers the narrative token-by-token via SSE callbacks

The DM prompt instructs the LLM to always use tools for mechanical resolution (dice rolls, skill checks, inventory changes) rather than making things up. This keeps the game fair while the narrative stays creative.

### axon-mind — The Rulebook

Prolog rules define game mechanics declaratively. The LLM queries them via a `query_rules` tool rather than reasoning about rules from memory.

```prolog
%% combat.pl
hits(Roll, TargetAC) :- Roll >= TargetAC.
critical(Roll) :- Roll >= 20.
fumble(Roll) :- Roll =< 1.

%% ability_check.pl
check_succeeds(Roll, Modifier, DC) :- Total is Roll + Modifier, Total >= DC.

%% prerequisites.pl
can_learn(Char, Spell) :-
    spell_requires_level(Spell, Lvl),
    char_level(Char, CharLvl),
    CharLvl >= Lvl.

can_equip(Char, Item) :-
    item_requires(Item, Attr, Min),
    char_attr(Char, Attr, Val),
    Val >= Min.

%% quest logic
quest_available(Q) :- \+ quest_completed(Q), forall(prerequisite(Q, P), quest_completed(P)).
```

Dynamic facts get asserted as game state changes:

```go
engine.Assert("char_level", "aldric", "5")
engine.Assert("char_attr", "aldric", "dex", "16")
engine.Assert("quest_completed", "find_amulet")
```

The LLM calls `query_rules("can_equip(aldric, dragon_slayer_sword).")` and gets a definitive answer. No hallucinated rules.

### axon-fact — The Save File

Every game action is an immutable event. The complete game state is reconstructable by replaying events.

**Streams:**

```
character:{id}     — CharacterCreated, AttributeChanged, LevelUp,
                     DamageTaken, Healed, ItemAcquired, ItemDropped,
                     SpellLearned, QuestAccepted, QuestCompleted

encounter:{id}     — EncounterStarted, TurnTaken, AttackResolved,
                     SpellCast, CreatureDefeated, EncounterEnded

world:{region}     — RegionEntered, EventTriggered, NPCSpawned,
                     EnvironmentChanged

campaign:{id}      — CampaignCreated, SessionStarted, SessionEnded,
                     MilestoneReached
```

**Projectors** build read models synchronously:

- **CharacterSheet** — current HP, inventory, level, abilities (from character events)
- **EncounterState** — initiative order, creature HP, active effects (from encounter events)
- **WorldState** — current region, active quests, world flags (from world events)
- **CampaignLog** — session summaries, milestones (from campaign events)

**Why event sourcing for an RPG:**

- **Save/Load** is free — replay events to reconstruct any point in time
- **Undo** — drop last N events, replay the rest
- **Audit** — complete history of every dice roll, every decision
- **Derived state** — XP totals, kill counts, quest completion stats computed from events
- **Multiplayer** — events publish to other instances via axon-nats

### axon-memo — NPC Memory

NPCs remember the player across sessions. After each NPC conversation, axon-memo extracts memories:

- **Episodic**: "Player helped defend the village from goblins"
- **Semantic**: "Player is a level 5 fighter who prefers diplomacy over combat"
- **Emotional**: "Player showed mercy to a captured bandit — positive benevolence shift"

The `recall_npc_memory` tool does vector search when the player talks to an NPC:

```go
// DM system prompt includes recalled memories:
// "The blacksmith remembers you brought back his stolen hammer last month.
//  He trusts you (benevolence: 0.8). He's heard you're looking for dragon scale."
```

Relationship metrics (ability/benevolence/integrity from the ABI trust model) drive NPC disposition — a shopkeeper who trusts you offers better prices.

Nightly consolidation merges granular memories into higher-level understanding, and personality synthesis evolves NPC behavior over time.

### axon-nats — Shared World (Multiplayer)

For multiplayer campaigns, `EventBus[GameEvent]` fans out world events across server instances:

```go
bus := nats.NewEventBus[WorldEvent](conn, nats.WithSubject("campaign.events"))

// Player A slays the dragon on Server 1
bus.Publish(WorldEvent{Type: "dragon_slain", Region: "mountain", Player: "Aldric"})

// Player B on Server 2 hears about it from an NPC next conversation
// The DM's context includes this world event
```

### axon-task — Background Processing

Queued workers handle async game operations:

- **NarrativeWorker** — generates session summaries after each play session
- **ConsolidationWorker** — nightly NPC memory consolidation
- **WorldTickWorker** — advances time-based world events (seasons change, NPCs move, rumors spread)

## Tool Definitions

The DM's toolbox — these are the `tool.ToolDef` instances wired into `loop.RunConfig`:

### Core Mechanics

| Tool | Purpose | Returns |
|------|---------|---------|
| `roll_dice` | Roll NdS+M (e.g. 2d6+3) | Roll result, breakdown |
| `check_skill` | Ability/skill check against DC | Pass/fail, roll detail, margin |
| `attack_roll` | Attack with weapon against target AC | Hit/miss/crit, damage if hit |
| `saving_throw` | Save against effect | Pass/fail, roll detail |
| `query_rules` | Query Prolog rulebook | True/false + variable bindings |

### State Management

| Tool | Purpose | Returns |
|------|---------|---------|
| `get_character` | Read character sheet | Stats, HP, inventory, abilities |
| `apply_damage` | Deal damage to character/creature | New HP, death saves if applicable |
| `heal` | Restore HP | New HP |
| `add_to_inventory` | Give item to character | Updated inventory |
| `remove_from_inventory` | Remove/consume item | Updated inventory |
| `update_quest` | Advance quest state | Quest status |
| `grant_xp` | Award experience, auto-level | XP total, level up details if any |

### World & NPCs

| Tool | Purpose | Returns |
|------|---------|---------|
| `recall_npc_memory` | What does this NPC know/feel about the player? | Memories, trust metrics |
| `get_location` | Current location details | Description, exits, NPCs, items |
| `move_to` | Change player location | New location description |
| `get_encounter` | Current encounter state | Initiative order, creature status |
| `check_time` | In-game time and conditions | Time of day, weather, season |

### Example Tool Implementation

```go
func rollDiceTool() tool.ToolDef {
    return tool.ToolDef{
        Name:        "roll_dice",
        Description: "Roll dice in NdS+M format. Use for all random outcomes.",
        Parameters: tool.ParameterSchema{
            Type:     "object",
            Required: []string{"notation"},
            Properties: map[string]tool.PropertySchema{
                "notation": {
                    Type:        "string",
                    Description: "Dice notation: 1d20, 2d6+3, 1d12-1",
                },
                "reason": {
                    Type:        "string",
                    Description: "Why rolling (shown in game log)",
                },
            },
        },
        Execute: func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
            notation, _ := args["notation"].(string)
            count, sides, modifier := parseDice(notation)

            rolls := make([]int, count)
            total := modifier
            for i := range rolls {
                rolls[i] = rand.Intn(sides) + 1
                total += rolls[i]
            }

            return tool.ToolResult{
                Content: fmt.Sprintf("Rolled %s: %d (dice: %v, modifier: %+d)",
                    notation, total, rolls, modifier),
            }
        },
    }
}

func queryRulesTool(engine *mind.Engine) tool.ToolDef {
    return tool.ToolDef{
        Name:        "query_rules",
        Description: "Query the game rulebook (Prolog). Use to check prerequisites, validate actions, resolve ambiguous rules.",
        Parameters: tool.ParameterSchema{
            Type:     "object",
            Required: []string{"goal"},
            Properties: map[string]tool.PropertySchema{
                "goal": {
                    Type:        "string",
                    Description: "Prolog goal to query, e.g. can_equip(aldric, longsword).",
                },
            },
        },
        Execute: func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
            goal, _ := args["goal"].(string)
            solutions, err := engine.Query(goal)
            if err != nil {
                return tool.ToolResult{Content: fmt.Sprintf("Rule query error: %v", err)}
            }
            if len(solutions) == 0 {
                return tool.ToolResult{Content: "No. (query has no solutions)"}
            }
            data, _ := mind.SolutionsJSON(solutions)
            return tool.ToolResult{Content: string(data)}
        },
    }
}
```

## System Prompt Structure

The DM's system prompt is assembled from layers:

```
1. DM Personality & Style
   "You are a dungeon master for a dark fantasy campaign.
    Your tone is vivid but concise. You favor player agency..."

2. Campaign Context (from campaign events)
   "Campaign: The Shattered Crown. Session 7.
    Party last left off in the Ironwood Forest..."

3. Current State (from projectors)
   "Aldric: Level 5 Fighter, 34/45 HP, carrying longsword + shield.
    Location: Tavern in Millhaven. Time: Evening, light rain."

4. Active Quests
   "- Find the stolen crown (active, lead: visit the hermit)
    - Clear the goblin cave (completed)"

5. NPC Memory Context (from axon-memo, when talking to NPCs)
   "Blacksmith Tormund: Trusts you (0.8). Remembers you returned
    his hammer. Knows you seek dragon scale armor."

6. Rules Reference
   "Use tools for ALL mechanical resolution. Never invent dice rolls.
    Always call check_skill for ability checks, attack_roll for combat.
    Query rules when unsure about prerequisites or constraints."

7. Tool Usage Guidelines
   "- roll_dice: any random outcome
    - check_skill: ability checks, skill checks
    - attack_roll: combat attacks
    - query_rules: rule lookups, prerequisite checks
    - get_character: before describing character state
    - apply_damage/heal: HP changes
    - recall_npc_memory: before voicing any recurring NPC"
```

## Implementation Plan

Each step is one commit-sized change.

### Phase 1: Foundation

1. **New module `axon-rpg`** — `go mod init`, basic types (Character, Creature, Item, Location)
2. **Dice engine** — `roll_dice` tool + dice notation parser + tests
3. **Character model** — event-sourced character sheet with projector (axon-fact)
4. **Prolog rulebook** — base rules (ability checks, combat, prerequisites) + `query_rules` tool

### Phase 2: Game Loop

5. **DM agent** — system prompt builder, tool wiring, `loop.Run` integration
6. **Skill check tool** — `check_skill` using dice engine + Prolog rules
7. **Combat tools** — `attack_roll`, `apply_damage`, `heal`, initiative tracking
8. **Inventory tools** — `add_to_inventory`, `remove_from_inventory`, encumbrance rules

### Phase 3: World

9. **Location model** — event-sourced locations, `get_location`, `move_to`
10. **Encounter model** — event-sourced encounters, turn tracking
11. **Quest system** — Prolog-driven prerequisites, `update_quest`, completion checks
12. **NPC memory integration** — `recall_npc_memory` tool wired to axon-memo

### Phase 4: Service

13. **HTTP handlers** — game session endpoints, SSE streaming (reuse axon-chat patterns)
14. **Campaign persistence** — session start/end, campaign-level events
15. **Multiplayer** — axon-nats for shared world events
16. **Eval plans** — axon-eval scenarios for testing DM behavior

## What Doesn't Need Building

The axon ecosystem already provides:

- HTTP server with graceful shutdown, health checks, metrics → **axon**
- SSE streaming with fan-out → **axon/sse**
- Conversation loop with tool dispatch and streaming → **axon-loop**
- Tool definition framework → **axon-tool**
- Event store with projectors (memory + postgres) → **axon-fact**
- Prolog inference engine → **axon-mind**
- NPC memory with vector search and relationship tracking → **axon-memo**
- Cross-instance event distribution → **axon-nats**
- Background task queue → **axon-task**
- LLM provider adapters (Ollama, Claude, GPT) → **axon-talk**
- Evaluation framework → **axon-eval**
- Auth middleware → **axon**

The RPG-specific code is: game types, dice parser, Prolog rule files, tool implementations, system prompt builder, and HTTP handlers. Everything else is composition.
