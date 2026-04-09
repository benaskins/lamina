package cairn

import (
	"fmt"
	"math/rand"

	game "github.com/benaskins/axon-game"
)

// Engine implements game.RulesEngine for Cairn.
type Engine struct{}

// NewEngine returns a new Cairn rules engine.
func NewEngine() *Engine { return &Engine{} }

func (e *Engine) Name() string { return "Cairn" }

func (e *Engine) RulesText() string {
	return `Cairn Rules:
Saves: roll d20 equal to or under STR/DEX/WIL. Natural 1 always succeeds; natural 20 always fails.
Combat: attacks auto-hit. Roll weapon die (d4/d6/d8/d10), subtract target armor (max 3). d12 if enhanced; d4 if impaired.
Multiple attackers vs one target: each rolls, keep highest.
HP represents luck, not health. At 0 HP: excess damage reduces STR. Make STR save or be incapacitated (critical damage).
STR 0 = dead. DEX 0 = paralysed. WIL 0 = delirious.
10 inventory slots. Bulky items take 2 slots. Full inventory (10 slots) = HP reduced to 0 and Deprived (cannot recover HP).
Magic: hold spellbook in both hands, read aloud, add 1 Fatigue slot. WIL save if deprived or in danger.
Morale: enemies make WIL save on first casualty or when reduced to half strength.
Reaction: 2d6 — 2 hostile, 3-5 wary, 6-8 curious, 9-11 kind, 12 helpful.`
}

func (e *Engine) Save(char game.Character, ability string, reason string) game.SaveResult {
	score := char.Ability(ability)
	rolled := rand.Intn(20) + 1

	var passed, critical bool
	switch {
	case rolled == 1:
		passed, critical = true, true
	case rolled == 20:
		passed, critical = false, true
	default:
		passed = rolled <= score
	}

	detail := fmt.Sprintf("%s save: rolled %d vs %s %d", reason, rolled, ability, score)
	if critical && passed {
		detail += " — critical success (natural 1)"
	} else if critical {
		detail += " — critical failure (natural 20)"
	} else if passed {
		detail += " — success"
	} else {
		detail += " — failure"
	}

	return game.SaveResult{
		Ability:  ability,
		Score:    score,
		Roll:     rolled,
		Passed:   passed,
		Critical: critical,
		Detail:   detail,
	}
}

func (e *Engine) Attack(opts game.AttackOpts) game.AttackResult {
	die := opts.WeaponDie
	switch opts.Modifier {
	case game.ModifierEnhanced:
		die = 12
	case game.ModifierImpaired:
		die = 4
	}

	rolled := rand.Intn(die) + 1
	armor := opts.Armor
	if armor > 3 {
		armor = 3
	}
	damage := rolled - armor
	if damage < 0 {
		damage = 0
	}

	detail := fmt.Sprintf("Attack: d%d rolled %d, armor %d, damage %d", die, rolled, armor, damage)

	return game.AttackResult{
		Die:    die,
		Roll:   rolled,
		Armor:  armor,
		Damage: damage,
		Detail: detail,
	}
}

func (e *Engine) ApplyDamage(char game.Character, amount int) game.DamageResult {
	oldHP := char.GetHP()
	newHP := oldHP - amount
	overflow := 0
	critical := false

	if newHP < 0 {
		overflow = -newHP
		newHP = 0
		critical = true
	}

	detail := fmt.Sprintf("%s: HP %d -> %d", char.GetName(), oldHP, newHP)
	if overflow > 0 {
		detail += fmt.Sprintf(", overflow %d hits STR (STR save required)", overflow)
	}

	return game.DamageResult{
		OldHP:    oldHP,
		NewHP:    newHP,
		Overflow: overflow,
		Critical: critical,
		Detail:   detail,
	}
}

func (e *Engine) Heal(char game.Character, amount int) game.HealResult {
	if char.IsDeprived() {
		return game.HealResult{
			OldHP:   char.GetHP(),
			NewHP:   char.GetHP(),
			Blocked: true,
			Detail:  fmt.Sprintf("%s is Deprived and cannot recover HP", char.GetName()),
		}
	}
	oldHP := char.GetHP()
	newHP := oldHP + amount
	if newHP > char.GetMaxHP() {
		newHP = char.GetMaxHP()
	}
	return game.HealResult{
		OldHP:  oldHP,
		NewHP:  newHP,
		Detail: fmt.Sprintf("%s: HP %d -> %d", char.GetName(), oldHP, newHP),
	}
}

// Reaction rolls 2d6 and returns an NPC disposition.
func (e *Engine) Reaction() game.ReactionResult {
	total := rand.Intn(6) + 1 + rand.Intn(6) + 1
	var disposition string
	switch {
	case total == 2:
		disposition = "hostile"
	case total <= 5:
		disposition = "wary"
	case total <= 8:
		disposition = "curious"
	case total <= 11:
		disposition = "kind"
	default:
		disposition = "helpful"
	}
	return game.ReactionResult{Roll: total, Disposition: disposition}
}
