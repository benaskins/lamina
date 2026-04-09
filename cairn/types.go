package cairn

import game "github.com/benaskins/axon-game"

// Re-exported types from axon-game for use by consumers of this package.

// AttackOpts configures an attack roll.
type AttackOpts = game.AttackOpts

// AttackModifier adjusts the attack die.
type AttackModifier = game.AttackModifier

// ModifierNormal, ModifierEnhanced, ModifierImpaired are the attack modifier constants.
const (
	ModifierNormal   = game.ModifierNormal
	ModifierEnhanced = game.ModifierEnhanced
	ModifierImpaired = game.ModifierImpaired
)

// AttackResult is the outcome of an attack.
type AttackResult = game.AttackResult

// DamageResult is the outcome of applying damage to a character.
type DamageResult = game.DamageResult

// HealResult is the outcome of healing.
type HealResult = game.HealResult

// SaveResult holds the outcome of a save attempt.
type SaveResult = game.SaveResult

// ReactionResult holds the outcome of an NPC reaction roll.
type ReactionResult = game.ReactionResult
