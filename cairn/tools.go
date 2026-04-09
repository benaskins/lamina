package cairn

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	fact "github.com/benaskins/axon-fact"
	game "github.com/benaskins/axon-game"
	tool "github.com/benaskins/axon-tool"
)

// AllTools returns all Cairn tool definitions wired to the given store and event store.
// es may be nil (tools will work without persistence).
func AllTools(store CharacterStore, es fact.EventStore) []tool.ToolDef {
	return []tool.ToolDef{
		RollDiceTool(),
		SaveTool(store),
		ReactionRollTool(),
		AttackTool(store, es),
		ApplyDamageTool(store, es),
		HealTool(store, es),
		AddItemTool(store, es),
		DropItemTool(store, es),
		AddFatigueTool(store, es),
		MoraleCheckTool(es),
		GetCharacterTool(store),
	}
}

// RollDiceTool rolls dice in NdS+M notation.
func RollDiceTool() tool.ToolDef {
	return tool.ToolDef{
		Name:        "roll_dice",
		Description: "Roll dice in NdS+M notation (e.g. 2d6+3, 1d20, d4). Use for any random outcome not covered by a specific mechanic.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"notation"},
			Properties: map[string]tool.PropertySchema{
				"notation": {Type: "string", Description: "Dice notation: 1d20, 2d6+3, d4"},
				"reason":   {Type: "string", Description: "Why rolling (for the game log)"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			notation, _ := args["notation"].(string)
			reason, _ := args["reason"].(string)
			n, s, mod := parseDiceNotation(notation)
			rolls := make([]int, n)
			total := mod
			for i := range rolls {
				rolls[i] = rand.Intn(s) + 1
				total += rolls[i]
			}
			result := fmt.Sprintf("Rolled %s: %d (dice: %v, modifier: %+d)", notation, total, rolls, mod)
			if reason != "" {
				result = reason + ": " + result
			}
			return tool.ToolResult{Content: result}
		},
	}
}

// SaveTool resolves a Cairn save (d20 under ability score).
func SaveTool(store CharacterStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "save",
		Description: "Make a Cairn save: roll d20 vs STR, DEX, or WIL. Roll equal to or under the score to succeed. Natural 1 always succeeds; natural 20 always fails. Use for all risky actions.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"character", "ability"},
			Properties: map[string]tool.PropertySchema{
				"character": {Type: "string", Description: "Character ID"},
				"ability":   {Type: "string", Description: "Ability to save against: STR, DEX, or WIL", Enum: []any{"STR", "DEX", "WIL"}},
				"reason":    {Type: "string", Description: "What the character is attempting"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			charID, _ := args["character"].(string)
			ability, _ := args["ability"].(string)
			reason, _ := args["reason"].(string)

			char, err := store.GetCharacter(tc.Ctx, charID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}

			engine := NewEngine()
			result := engine.Save(char, ability, reason)
			return tool.ToolResult{Content: result.Detail}
		},
	}
}

// ReactionRollTool rolls 2d6 for NPC disposition.
func ReactionRollTool() tool.ToolDef {
	return tool.ToolDef{
		Name:        "reaction_roll",
		Description: "Roll 2d6 for NPC disposition when encountering a new NPC. 2=hostile, 3-5=wary, 6-8=curious, 9-11=kind, 12=helpful.",
		Parameters: tool.ParameterSchema{
			Type:       "object",
			Properties: map[string]tool.PropertySchema{},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			engine := NewEngine()
			result := engine.Reaction()
			return tool.ToolResult{Content: fmt.Sprintf("Reaction: rolled %d — %s", result.Roll, result.Disposition)}
		},
	}
}

// AttackTool resolves a Cairn attack.
func AttackTool(store CharacterStore, es fact.EventStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "attack",
		Description: "Resolve a Cairn attack. Attacks auto-hit. Roll weapon die, subtract target armor (max 3). Set enhanced=true for ambush/strong advantage (rolls d12). Set impaired=true for disadvantage (rolls d4). Multiple attackers vs one target: call separately, the Warden keeps the highest.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"weapon_die", "target"},
			Properties: map[string]tool.PropertySchema{
				"weapon_die": {Type: "integer", Description: "Weapon damage die sides: 4, 6, 8, or 10", Enum: []any{4, 6, 8, 10}},
				"target":     {Type: "string", Description: "Target character ID"},
				"enhanced":   {Type: "boolean", Description: "Roll d12 instead of weapon die (ambush, strong advantage)"},
				"impaired":   {Type: "boolean", Description: "Roll d4 instead of weapon die (disarmed, prone, disadvantage)"},
				"attacker":   {Type: "string", Description: "Attacker name (for the log)"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			targetID, _ := args["target"].(string)
			weaponDie, _ := args["weapon_die"].(float64)
			enhanced, _ := args["enhanced"].(bool)
			impaired, _ := args["impaired"].(bool)

			target, err := store.GetCharacter(tc.Ctx, targetID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}

			mod := game.ModifierNormal
			if enhanced {
				mod = game.ModifierEnhanced
			} else if impaired {
				mod = game.ModifierImpaired
			}

			engine := NewEngine()
			result := engine.Attack(game.AttackOpts{
				WeaponDie: int(weaponDie),
				Armor:     target.ArmorValue,
				Modifier:  mod,
			})

			_ = emitEvent(tc.Ctx, es, "encounter:"+targetID, AttackResolved{
				TargetID: targetID,
				Die:      result.Die,
				Roll:     result.Roll,
				Armor:    result.Armor,
				Damage:   result.Damage,
			})

			return tool.ToolResult{Content: result.Detail}
		},
	}
}

// ApplyDamageTool applies damage to a character's HP (with STR overflow handling).
func ApplyDamageTool(store CharacterStore, es fact.EventStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "apply_damage",
		Description: "Apply damage to a character. HP absorbs first; if HP hits 0, excess reduces STR and triggers a STR save. Call after attack resolves. If damage = 0, still call to record the near-miss.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"character", "damage"},
			Properties: map[string]tool.PropertySchema{
				"character": {Type: "string", Description: "Character ID"},
				"damage":    {Type: "integer", Description: "Damage amount"},
				"source":    {Type: "string", Description: "Source of damage (for the log)"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			charID, _ := args["character"].(string)
			damage, _ := args["damage"].(float64)
			source, _ := args["source"].(string)
			if source == "" {
				source = "unknown"
			}

			char, err := store.GetCharacter(tc.Ctx, charID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}

			engine := NewEngine()
			result := engine.ApplyDamage(char, int(damage))

			stream := "character:" + charID
			_ = emitEvent(tc.Ctx, es, stream, DamageTaken{
				Amount:   int(damage),
				OldHP:    result.OldHP,
				NewHP:    result.NewHP,
				Overflow: result.Overflow,
			})

			if result.Overflow > 0 {
				_ = emitEvent(tc.Ctx, es, stream, STRLost{
					Amount: result.Overflow,
					Source: source,
				})
			}

			return tool.ToolResult{Content: result.Detail}
		},
	}
}

// HealTool restores HP to a character (blocked if Deprived).
func HealTool(store CharacterStore, es fact.EventStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "heal",
		Description: "Restore HP to a character. Healing is blocked if the character is Deprived (inventory full). Characters cannot exceed their maximum HP.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"character", "amount"},
			Properties: map[string]tool.PropertySchema{
				"character": {Type: "string", Description: "Character ID"},
				"amount":    {Type: "integer", Description: "HP to restore"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			charID, _ := args["character"].(string)
			amount, _ := args["amount"].(float64)

			char, err := store.GetCharacter(tc.Ctx, charID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}

			engine := NewEngine()
			result := engine.Heal(char, int(amount))

			if !result.Blocked {
				_ = emitEvent(tc.Ctx, es, "character:"+charID, Healed{
					Amount: int(amount),
					OldHP:  result.OldHP,
					NewHP:  result.NewHP,
				})
			}

			return tool.ToolResult{Content: result.Detail}
		},
	}
}

// AddItemTool adds an item to a character's inventory.
func AddItemTool(store CharacterStore, es fact.EventStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "add_item",
		Description: "Add an item to a character's inventory. Bulky items take 2 slots. If all 10 slots fill, HP drops to 0 and character becomes Deprived.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"character", "item"},
			Properties: map[string]tool.PropertySchema{
				"character": {Type: "string", Description: "Character ID"},
				"item":      {Type: "string", Description: "Item name"},
				"bulky":     {Type: "boolean", Description: "Item is bulky (takes 2 slots)"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			charID, _ := args["character"].(string)
			item, _ := args["item"].(string)
			bulky, _ := args["bulky"].(bool)

			char, err := store.GetCharacter(tc.Ctx, charID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}

			slotCost := 1
			if bulky {
				slotCost = 2
			}
			available := 10 - char.SlotsUsed()
			if available < slotCost {
				return tool.ToolResult{Content: fmt.Sprintf("%s has no room for %s (%d slots needed, %d available)", char.CharName, item, slotCost, available)}
			}

			_ = emitEvent(tc.Ctx, es, "character:"+charID, ItemAdded{Item: item, Bulky: bulky})

			return tool.ToolResult{Content: fmt.Sprintf("%s added to %s's inventory (%d/%d slots used)", item, char.CharName, char.SlotsUsed()+slotCost, 10)}
		},
	}
}

// DropItemTool removes an item from a character's inventory.
func DropItemTool(store CharacterStore, es fact.EventStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "drop_item",
		Description: "Remove an item from a character's inventory.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"character", "item"},
			Properties: map[string]tool.PropertySchema{
				"character": {Type: "string", Description: "Character ID"},
				"item":      {Type: "string", Description: "Item name to drop"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			charID, _ := args["character"].(string)
			item, _ := args["item"].(string)

			char, err := store.GetCharacter(tc.Ctx, charID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}

			for _, slot := range char.Inventory {
				if strings.EqualFold(slot.Item, item) {
					_ = emitEvent(tc.Ctx, es, "character:"+charID, ItemDropped{Item: slot.Item})
					return tool.ToolResult{Content: fmt.Sprintf("%s dropped from %s's inventory", slot.Item, char.CharName)}
				}
			}
			return tool.ToolResult{Content: fmt.Sprintf("%s is not in %s's inventory", item, char.CharName)}
		},
	}
}

// AddFatigueTool adds a Fatigue token to a character's inventory (from spellcasting or deprivation).
func AddFatigueTool(store CharacterStore, es fact.EventStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "add_fatigue",
		Description: "Add a Fatigue token to a character's inventory (occupies 1 slot). Triggered by spellcasting or severe deprivation.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"character"},
			Properties: map[string]tool.PropertySchema{
				"character": {Type: "string", Description: "Character ID"},
				"source":    {Type: "string", Description: "Why fatigue was added (spellcasting, deprivation, etc.)"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			charID, _ := args["character"].(string)
			source, _ := args["source"].(string)
			if source == "" {
				source = "unknown"
			}

			char, err := store.GetCharacter(tc.Ctx, charID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}

			_ = emitEvent(tc.Ctx, es, "character:"+charID, FatigueAdded{Source: source})

			return tool.ToolResult{Content: fmt.Sprintf("Fatigue added to %s's inventory (%d/%d slots, from %s)", char.CharName, char.SlotsUsed()+1, 10, source)}
		},
	}
}

// MoraleCheckTool checks if enemies hold or flee.
func MoraleCheckTool(es fact.EventStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "morale_check",
		Description: "Check if enemies hold or flee. Trigger when the first enemy ally falls, or when the group drops to half strength. Lone enemies also check at 0 HP.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"creature_type", "wil"},
			Properties: map[string]tool.PropertySchema{
				"creature_type": {Type: "string", Description: "Type of creature (for the log)"},
				"wil":           {Type: "integer", Description: "WIL score of the creature group"},
				"encounter_id":  {Type: "string", Description: "Encounter ID"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			creatureType, _ := args["creature_type"].(string)
			wil, _ := args["wil"].(float64)
			encounterID, _ := args["encounter_id"].(string)

			rolled := rand.Intn(20) + 1
			fled := rolled > int(wil)

			stream := "encounter:" + encounterID
			_ = emitEvent(tc.Ctx, es, stream, MoraleChecked{
				CreatureType: creatureType,
				WIL:          int(wil),
				Roll:         rolled,
				Fled:         fled,
			})

			if fled {
				return tool.ToolResult{Content: fmt.Sprintf("%s morale breaks (rolled %d vs WIL %d) — they flee", creatureType, rolled, int(wil))}
			}
			return tool.ToolResult{Content: fmt.Sprintf("%s holds (rolled %d vs WIL %d)", creatureType, rolled, int(wil))}
		},
	}
}

// GetCharacterTool reads a character sheet.
func GetCharacterTool(store CharacterStore) tool.ToolDef {
	return tool.ToolDef{
		Name:        "get_character",
		Description: "Read a character's current sheet: stats, HP, inventory, scars, and conditions. Call before describing a character's state.",
		Parameters: tool.ParameterSchema{
			Type:     "object",
			Required: []string{"character"},
			Properties: map[string]tool.PropertySchema{
				"character": {Type: "string", Description: "Character ID"},
			},
		},
		Execute: func(tc *tool.ToolContext, args map[string]any) tool.ToolResult {
			charID, _ := args["character"].(string)
			char, err := store.GetCharacter(tc.Ctx, charID)
			if err != nil {
				return tool.ToolResult{Content: fmt.Sprintf("Error: %v", err)}
			}
			return tool.ToolResult{Content: formatSheet(char)}
		},
	}
}

func formatSheet(c *Sheet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)\n", c.CharName, c.Background)
	fmt.Fprintf(&b, "STR %d/%d  DEX %d/%d  WIL %d/%d\n", c.STR, c.MaxSTR, c.DEX, c.MaxDEX, c.WIL, c.MaxWIL)
	fmt.Fprintf(&b, "HP %d/%d  Armor %d  Gold %d\n", c.CurrentHP, c.MaxHP, c.ArmorValue, c.Gold)
	if c.IsDeprived() {
		fmt.Fprintf(&b, "DEPRIVED (cannot recover HP)\n")
	}
	if c.Dead {
		fmt.Fprintf(&b, "DEAD\n")
	}
	fmt.Fprintf(&b, "Inventory (%d/10 slots):\n", c.SlotsUsed())
	for _, slot := range c.Inventory {
		tag := ""
		if slot.Bulky {
			tag = " (bulky)"
		}
		if slot.Fatigue {
			tag = " (fatigue)"
		}
		fmt.Fprintf(&b, "  - %s%s\n", slot.Item, tag)
	}
	if len(c.Scars) > 0 {
		fmt.Fprintf(&b, "Scars:\n")
		for _, scar := range c.Scars {
			fmt.Fprintf(&b, "  - %s (from %s)\n", scar.Description, scar.Source)
		}
	}
	return b.String()
}

// parseDiceNotation parses "NdS+M" or "dS" into components.
func parseDiceNotation(notation string) (n, sides, modifier int) {
	n = 1
	sides = 6
	modifier = 0
	notation = strings.TrimSpace(notation)

	// Handle optional N prefix
	dIdx := strings.Index(notation, "d")
	if dIdx < 0 {
		return
	}
	if dIdx > 0 {
		fmt.Sscanf(notation[:dIdx], "%d", &n)
	}

	rest := notation[dIdx+1:]
	plusIdx := strings.Index(rest, "+")
	minusIdx := strings.Index(rest, "-")

	switch {
	case plusIdx >= 0:
		fmt.Sscanf(rest[:plusIdx], "%d", &sides)
		fmt.Sscanf(rest[plusIdx+1:], "%d", &modifier)
	case minusIdx >= 0:
		fmt.Sscanf(rest[:minusIdx], "%d", &sides)
		fmt.Sscanf(rest[minusIdx:], "%d", &modifier)
	default:
		fmt.Sscanf(rest, "%d", &sides)
	}
	return
}

// toolContext is a context.Context wrapper used for tool execution.
// It provides access to the request context within tool Execute functions.
type toolContext struct {
	context.Context
}
